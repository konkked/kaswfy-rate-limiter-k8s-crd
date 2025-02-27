package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RateLimitConfigSpec defines the desired state of RateLimitConfig
type RateLimitConfigSpec struct {
	// DeploymentName is the target Deployment for rate limiting
	DeploymentName string `json:"deploymentName"`
	// MaxRequestsByIpRps limits requests per IP (optional)
	MaxRequestsByIpRps *int32 `json:"maxRequestsByIpRps,omitempty"`
	// MaxRequestsByUserRps limits requests per user (optional, requires user ID header)
	MaxRequestsByUserRps *int32 `json:"maxRequestsByUserRps,omitempty"`
	// MaxRequestsByLikeRouteRps limits requests per similar route pattern (optional)
	MaxRequestsByLikeRouteRps *int32 `json:"maxRequestsByLikeRouteRps,omitempty"`
	// EnvoyPort specifies the port Envoy listens on (optional, defaults to 8080)
	EnvoyPort *int32 `json:"envoyPort,omitempty"`
	// ClusterName is the name of the cluster
	ClusterName string `json:"clusterName"`
	// HeaderRateLimits defines rate limits based on headers
	HeaderRateLimits []HeaderRateLimit `json:"headerRateLimits,omitempty"`
}

// HeaderRateLimit defines rate limits for a specific header
type HeaderRateLimit struct {
	// HeaderName is the name of the header
	HeaderName string `json:"headerName"`
	// Rps is the rate limit in requests per second
	Rps int32 `json:"rps"`
}

// RateLimitConfigStatus defines the observed state of RateLimitConfig
type RateLimitConfigStatus struct {
	// Applied indicates if rate limiting has been enforced
	Applied bool `json:"applied"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type RateLimitConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RateLimitConfigSpec   `json:"spec,omitempty"`
	Status RateLimitConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type RateLimitConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RateLimitConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RateLimitConfig{}, &RateLimitConfigList{})
}
