package postgres

import (
	"fmt"
	"sync"

	hookapi "github.com/appscode/kutil/admission/api"
	meta_util "github.com/appscode/kutil/meta"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	cs "github.com/kubedb/apimachinery/client/clientset/versioned"
	"github.com/kubedb/kubedb-server/pkg/admission/util"
	pgv "github.com/kubedb/postgres/pkg/validator"
	admission "k8s.io/api/admission/v1beta1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type PostgresValidator struct {
	client      kubernetes.Interface
	extClient   cs.Interface
	lock        sync.RWMutex
	initialized bool
}

var _ hookapi.AdmissionHook = &PostgresValidator{}

func (a *PostgresValidator) Resource() (plural schema.GroupVersionResource, singular string) {
	return schema.GroupVersionResource{
			Group:    "admission.kubedb.com",
			Version:  "v1alpha1",
			Resource: "postgresreviews",
		},
		"postgresreview"
}

func (a *PostgresValidator) Initialize(config *rest.Config, stopCh <-chan struct{}) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	a.initialized = true

	var err error
	if a.client, err = kubernetes.NewForConfig(config); err != nil {
		return err
	}
	if a.extClient, err = cs.NewForConfig(config); err != nil {
		return err
	}
	return nil
}

func (a *PostgresValidator) Admit(req *admission.AdmissionRequest) *admission.AdmissionResponse {
	status := &admission.AdmissionResponse{}

	if (req.Operation != admission.Create && req.Operation != admission.Update && req.Operation != admission.Delete) ||
		len(req.SubResource) != 0 ||
		req.Kind.Group != api.SchemeGroupVersion.Group ||
		req.Kind.Kind != api.ResourceKindPostgres {
		status.Allowed = true
		return status
	}

	a.lock.RLock()
	defer a.lock.RUnlock()
	if !a.initialized {
		return hookapi.StatusUninitialized()
	}

	switch req.Operation {
	case admission.Delete:
		// req.Object.Raw = nil, so read from kubernetes
		obj, err := a.extClient.KubedbV1alpha1().Postgreses(req.Namespace).Get(req.Name, metav1.GetOptions{})
		if err != nil && !kerr.IsNotFound(err) {
			return hookapi.StatusInternalServerError(err)
		} else if err == nil && obj.Spec.DoNotPause {
			return hookapi.StatusBadRequest(fmt.Errorf(`postgres "%s" can't be paused. To continue delete, unset spec.doNotPause and retry`, req.Name))
		}
	default:
		obj, err := meta_util.UnmarshalToJSON(req.Object.Raw, api.SchemeGroupVersion)
		if err != nil {
			return hookapi.StatusBadRequest(err)
		}
		if req.Operation == admission.Update && !util.IsKubeDBOperator(req.UserInfo) {
			// validate changes made by user
			oldObject, err := meta_util.UnmarshalToJSON(req.OldObject.Raw, api.SchemeGroupVersion)
			if err != nil {
				return hookapi.StatusBadRequest(err)
			}
			if err := util.ValidateUpdate(obj, oldObject, req.Kind.Kind); err != nil {
				return hookapi.StatusBadRequest(fmt.Errorf("%v", err))
			}
		}
		// validate database specs
		if err = pgv.ValidatePostgres(a.client, a.extClient.KubedbV1alpha1(), obj.(*api.Postgres)); err != nil {
			return hookapi.StatusForbidden(err)
		}
	}

	status.Allowed = true
	return status
}
