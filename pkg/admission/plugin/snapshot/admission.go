package snapshot

import (
	"net/http"
	"sync"

	"github.com/appscode/kutil/meta"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	amv "github.com/kubedb/apimachinery/pkg/validator"
	hookapi "github.com/kubedb/kubedb-server/pkg/admission/api"
	admission "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type SnapshotValidator struct {
	client      kubernetes.Interface
	lock        sync.RWMutex
	initialized bool
}

var _ hookapi.AdmissionHook = &SnapshotValidator{}

func (a *SnapshotValidator) Resource() (plural schema.GroupVersionResource, singular string) {
	return schema.GroupVersionResource{
			Group:    "admission.kubedb.com",
			Version:  "v1alpha1",
			Resource: "snapshotreviews",
		},
		"snapshotreview"
}

func (a *SnapshotValidator) Initialize(config *rest.Config, stopCh <-chan struct{}) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	a.initialized = true

	var err error
	a.client, err = kubernetes.NewForConfig(config)
	return err
}

func (a *SnapshotValidator) Admit(req *admission.AdmissionRequest) *admission.AdmissionResponse {
	status := &admission.AdmissionResponse{}

	if (req.Operation != admission.Create && req.Operation != admission.Update) ||
		len(req.SubResource) != 0 ||
		req.Kind.Group != api.SchemeGroupVersion.Group ||
		req.Kind.Kind != api.ResourceKindSnapshot {
		status.Allowed = true
		return status
	}

	a.lock.RLock()
	defer a.lock.RUnlock()
	if !a.initialized {
		status.Allowed = false
		status.Result = &metav1.Status{
			Status: metav1.StatusFailure, Code: http.StatusInternalServerError, Reason: metav1.StatusReasonInternalError,
			Message: "not initialized",
		}
		return status
	}

	obj, err := meta.UnmarshalToJSON(req.Object.Raw, api.SchemeGroupVersion)
	if err != nil {
		status.Allowed = false
		status.Result = &metav1.Status{
			Status: metav1.StatusFailure, Code: http.StatusBadRequest, Reason: metav1.StatusReasonBadRequest,
			Message: err.Error(),
		}
		return status
	}

	err = a.check(req.Operation, obj)
	if err != nil {
		status.Allowed = false
		status.Result = &metav1.Status{
			Status: metav1.StatusFailure, Code: http.StatusForbidden, Reason: metav1.StatusReasonForbidden,
			Message: err.Error(),
		}
		return status
	}

	status.Allowed = true
	return status
}

func (a *SnapshotValidator) check(op admission.Operation, in runtime.Object) error {
	if op == admission.Delete {
		return nil
	}
	obj := in.(*api.Snapshot)
	return amv.ValidateSnapshotSpec(a.client, obj.Spec.SnapshotStorageSpec, obj.Namespace)
}
