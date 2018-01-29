package plugin

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/appscode/kutil/meta"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	amv "github.com/kubedb/apimachinery/pkg/validator"
	esv "github.com/kubedb/elasticsearch/pkg/validator"
	memv "github.com/kubedb/memcached/pkg/validator"
	mgv "github.com/kubedb/mongodb/pkg/validator"
	msv "github.com/kubedb/mysql/pkg/validator"
	pgv "github.com/kubedb/postgres/pkg/validator"
	rdv "github.com/kubedb/redis/pkg/validator"
	admission "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type AdmissionHook struct {
	client      kubernetes.Interface
	lock        sync.RWMutex
	initialized bool
}

func (a *AdmissionHook) ValidatingResource() (plural schema.GroupVersionResource, singular string) {
	return schema.GroupVersionResource{
			Group:    "admission.kubedb.com",
			Version:  "v1alpha1",
			Resource: "reviews",
		},
		"review"
}

func (a *AdmissionHook) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	a.initialized = true

	shallowClientConfigCopy := *kubeClientConfig
	var err error
	a.client, err = kubernetes.NewForConfig(&shallowClientConfigCopy)
	return err
}

func (a *AdmissionHook) Validate(req *admission.AdmissionRequest) *admission.AdmissionResponse {
	status := &admission.AdmissionResponse{}
	supportedKinds := sets.NewString(
		api.ResourceKindElasticsearch,
		api.ResourceKindMemcached,
		api.ResourceKindMongoDB,
		api.ResourceKindMySQL,
		api.ResourceKindPostgres,
		api.ResourceKindRedis,
		api.ResourceKindSnapshot,
	)

	if (req.Operation != admission.Create && req.Operation != admission.Update && req.Operation != admission.Delete) ||
		len(req.SubResource) != 0 ||
		req.Kind.Group != api.SchemeGroupVersion.Group ||
		!supportedKinds.Has(req.Kind.Kind) {
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

func (a *AdmissionHook) check(op admission.Operation, in runtime.Object) error {
	switch in.GetObjectKind().GroupVersionKind().Kind {
	case api.ResourceKindElasticsearch:
		obj := in.(*api.Elasticsearch)
		if op == admission.Delete {
			if obj.Spec.DoNotPause {
				return fmt.Errorf(`Elasticsearch %s can't be paused. To continue, unset spec.doNotPause and retry`, obj.Name)
			}
		} else {
			return esv.ValidateElasticsearch(a.client, obj, nil)
		}
	case api.ResourceKindPostgres:
		obj := in.(*api.Postgres)
		if op == admission.Delete {
			if obj.Spec.DoNotPause {
				return fmt.Errorf(`Postgres %s can't be paused. To continue delete, unset spec.doNotPause and retry`, obj.Name)
			}
		} else {
			return pgv.ValidatePostgres(a.client, obj, nil)
		}
	case api.ResourceKindMySQL:
		obj := in.(*api.MySQL)
		if op == admission.Delete {
			if obj.Spec.DoNotPause {
				return fmt.Errorf(`MySQL %s can't be paused. To continue delete, unset spec.doNotPause and retry`, obj.Name)
			}
		} else {
			return msv.ValidateMySQL(a.client, obj, nil)
		}
	case api.ResourceKindMongoDB:
		obj := in.(*api.MongoDB)
		if op == admission.Delete {
			if obj.Spec.DoNotPause {
				return fmt.Errorf(`MongoDB %s can't be paused. To continue delete, unset spec.doNotPause and retry`, obj.Name)
			}
		} else {
			return mgv.ValidateMongoDB(a.client, obj, nil)
		}
	case api.ResourceKindRedis:
		obj := in.(*api.Redis)
		if op == admission.Delete {
			if obj.Spec.DoNotPause {
				return fmt.Errorf(`Redis %s can't be paused. To continue delete, unset spec.doNotPause and retry`, obj.Name)
			}
		} else {
			return rdv.ValidateRedis(a.client, obj, nil)
		}
	case api.ResourceKindMemcached:
		obj := in.(*api.Memcached)
		if op == admission.Delete {
			if obj.Spec.DoNotPause {
				return fmt.Errorf(`Memcached %s can't be paused. To continue delete, unset spec.doNotPause and retry`, obj.Name)
			}
		} else {
			return memv.ValidateMemcached(a.client, obj, nil)
		}
	case api.ResourceKindSnapshot:
		if op == admission.Delete {
			return nil
		}
		obj := in.(*api.Snapshot)
		return amv.ValidateSnapshotSpec(a.client, obj.Spec.SnapshotStorageSpec, obj.Namespace)
	}
	return nil
}
