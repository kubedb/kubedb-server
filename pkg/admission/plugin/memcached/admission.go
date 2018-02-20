package memcached

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/appscode/kutil/meta"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	hookapi "github.com/kubedb/apiserver/pkg/admission/api"
	memv "github.com/kubedb/memcached/pkg/validator"
	admission "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type MemcachedValidator struct {
	client      kubernetes.Interface
	lock        sync.RWMutex
	initialized bool
}

var _ hookapi.AdmissionHook = &MemcachedValidator{}

func (a *MemcachedValidator) Resource() (plural schema.GroupVersionResource, singular string) {
	return schema.GroupVersionResource{
			Group:    "admission.kubedb.com",
			Version:  "v1alpha1",
			Resource: "memcachedreviews",
		},
		"memcachedreview"
}

func (a *MemcachedValidator) Initialize(config *rest.Config, stopCh <-chan struct{}) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	a.initialized = true

	shallowCopy := *config
	var err error
	a.client, err = kubernetes.NewForConfig(&shallowCopy)
	return err
}

func (a *MemcachedValidator) Admit(req *admission.AdmissionRequest) *admission.AdmissionResponse {
	status := &admission.AdmissionResponse{}

	if (req.Operation != admission.Create && req.Operation != admission.Update && req.Operation != admission.Delete) ||
		len(req.SubResource) != 0 ||
		req.Kind.Group != api.SchemeGroupVersion.Group ||
		req.Kind.Kind != api.ResourceKindMemcached {
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

func (a *MemcachedValidator) check(op admission.Operation, in runtime.Object) error {
	obj := in.(*api.Memcached)
	if op == admission.Delete {
		if obj.Spec.DoNotPause {
			return fmt.Errorf(`Memcached %s can't be paused. To continue delete, unset spec.doNotPause and retry`, obj.Name)
		}
	}
	return memv.ValidateMemcached(a.client, obj)
}
