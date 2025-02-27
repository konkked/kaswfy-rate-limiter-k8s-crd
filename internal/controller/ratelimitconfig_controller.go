package controller

import (
	"context"
	"fmt"
	"strings" // Added for strings.Join

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kaswfyv1 "kaswfy.io/rate-limiter-operator/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type RateLimitConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kaswfy.io,resources=ratelimitconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kaswfy.io,resources=ratelimitconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch

func (r *RateLimitConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	config := &kaswfyv1.RateLimitConfig{}
	if err := r.Get(ctx, req.NamespacedName, config); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: config.Namespace, Name: config.Spec.DeploymentName}, deployment); err != nil {
		log.Error(err, "Failed to get Deployment", "name", config.Spec.DeploymentName)
		return ctrl.Result{}, err
	}

	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == "envoy-sidecar" {
			if !config.Status.Applied {
				config.Status.Applied = true
				if err := r.Status().Update(ctx, config); err != nil {
					return ctrl.Result{}, err
				}
			}
			return ctrl.Result{}, nil
		}
	}

	// Determine Envoy port (default to 8080 if unset)
	envoyPort := int32(8080)
	if config.Spec.EnvoyPort != nil {
		envoyPort = *config.Spec.EnvoyPort
	}

	// Create or update Envoy ConfigMap
	configMapName := fmt.Sprintf("%s-ratelimit-config", config.Name)
	envoyConfig := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: config.Namespace,
		},
		Data: map[string]string{
			"envoy.yaml": generateEnvoyConfig(config, envoyPort),
		},
	}
	if err := ctrl.SetControllerReference(config, envoyConfig, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.Create(ctx, envoyConfig); err != nil {
		if client.IgnoreAlreadyExists(err) != nil {
			log.Error(err, "Failed to create ConfigMap", "name", configMapName)
			return ctrl.Result{}, err
		}
		existingConfig := &corev1.ConfigMap{}
		if err := r.Get(ctx, client.ObjectKey{Namespace: config.Namespace, Name: configMapName}, existingConfig); err == nil {
			existingConfig.Data["envoy.yaml"] = generateEnvoyConfig(config, envoyPort)
			if err := r.Update(ctx, existingConfig); err != nil {
				log.Error(err, "Failed to update ConfigMap", "name", configMapName)
				return ctrl.Result{}, err
			}
		}
	}

	// Inject Envoy sidecar
	deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, corev1.Container{
		Name:    "envoy-sidecar",
		Image:   "envoyproxy/envoy:v1.31-latest",
		Command: []string{"envoy", "-c", "/etc/envoy/envoy.yaml"},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "envoy-config", MountPath: "/etc/envoy"},
		},
		Ports: []corev1.ContainerPort{{ContainerPort: envoyPort}},
	})
	deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: "envoy-config",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
			},
		},
	})

	if err := r.Update(ctx, deployment); err != nil {
		log.Error(err, "Failed to update Deployment", "name", config.Spec.DeploymentName)
		return ctrl.Result{}, err
	}

	config.Status.Applied = true
	if err := r.Status().Update(ctx, config); err != nil {
		return ctrl.Result{}, err
	}
	log.Info("Rate limiting sidecar injected", "deployment", config.Spec.DeploymentName)
	return ctrl.Result{}, nil
}

func generateEnvoyConfig(config *kaswfyv1.RateLimitConfig, envoyPort int32) string {
	rateLimits := []string{}
	if config.Spec.MaxRequestsByIpRps != nil {
		rateLimits = append(rateLimits, fmt.Sprintf(`- actions:
            - remote_address: {}
          limit:
            requests_per_time_unit: %d
            time_unit: SECOND`, *config.Spec.MaxRequestsByIpRps))
	}
	if config.Spec.MaxRequestsByUserRps != nil {
		rateLimits = append(rateLimits, fmt.Sprintf(`- actions:
            - header_value:
                header_name: "X-User-ID"
          limit:
            requests_per_time_unit: %d
            time_unit: SECOND`, *config.Spec.MaxRequestsByUserRps))
	}
	if config.Spec.MaxRequestsByLikeRouteRps != nil {
		rateLimits = append(rateLimits, fmt.Sprintf(`- actions:
            - request_headers:
                header_name: ":path"
                descriptor_key: "route"
          limit:
            requests_per_time_unit: %d
            time_unit: SECOND`, *config.Spec.MaxRequestsByLikeRouteRps))
	}
	for _, headerRateLimit := range config.Spec.HeaderRateLimits {
		rateLimits = append(rateLimits, fmt.Sprintf(`- actions:
            - header_value:
                header_name: "%s"
          limit:
            requests_per_time_unit: %d
            time_unit: SECOND`, headerRateLimit.HeaderName, headerRateLimit.Rps))
	}

	rateLimitsStr := ""
	if len(rateLimits) > 0 {
		rateLimitsStr = "rate_limits:\n        " + strings.Join(rateLimits, "\n        ")
	}

	return fmt.Sprintf(`static_resources:
listeners:
- address:
    socket_address:
      address: 0.0.0.0
      port_value: %d
  filter_chains:
  - filters:
    - name: envoy.filters.network.http_connection_manager
      typed_config:
        "@type": "type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager"
        stat_prefix: ingress_http
        route_config:
          virtual_hosts:
          - name: backend
            domains: ["*"]
            routes:
            - match: {prefix: "/"}
              route: {cluster: "%s"}
              %s
        http_filters:
        - name: envoy.filters.http.ratelimit
          typed_config:
            "@type": "type.googleapis.com/envoy.extensions.filters.http.ratelimit.v3.RateLimit"
            domain: "miniflex"
clusters:
- name: %s
  connect_timeout: 0.25s
  type: logical_dns
  lb_policy: round_robin
  load_assignment:
    cluster_name: %s
    endpoints:
    - lb_endpoints:
      - endpoint:
          address:
            socket_address:
              address: 127.0.0.1
              port_value: 3000`, envoyPort, config.Spec.ClusterName, rateLimitsStr, config.Spec.ClusterName, config.Spec.ClusterName)
}

func (r *RateLimitConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kaswfyv1.RateLimitConfig{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
