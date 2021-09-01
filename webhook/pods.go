package webhook

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// deprecated

type PodCollector struct {
	Client  client.Client
	decoder *admission.Decoder
}

// podValidator admits a pod if a specific annotation exists.
func (v *PodCollector) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}

	err := v.decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	return admission.Allowed("")
}

// InjectDecoder injects the decoder.
func (v *PodCollector) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
