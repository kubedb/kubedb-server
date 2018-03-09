package memcached

import (
	"net/http"
	"os"
	"testing"

	kubeMon "github.com/appscode/kube-mon/api"
	"github.com/appscode/kutil/meta"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	extFake "github.com/kubedb/apimachinery/client/clientset/versioned/fake"
	"github.com/kubedb/apimachinery/client/clientset/versioned/scheme"
	"github.com/kubedb/kubedb-server/pkg/admission/util"
	admission "k8s.io/api/admission/v1beta1"
	authenticationV1 "k8s.io/api/authentication/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clientSetScheme "k8s.io/client-go/kubernetes/scheme"
)

func init() {
	scheme.AddToScheme(clientSetScheme.Scheme)
	os.Setenv(util.EnvSvcAccountName, "kubedb-operator")
	os.Setenv("KUBE_NAMESPACE", "kube-system")
}

var requestKind = metaV1.GroupVersionKind{
	Group:   api.SchemeGroupVersion.Group,
	Version: api.SchemeGroupVersion.Version,
	Kind:    api.ResourceKindMemcached,
}

func TestMemcachedValidator_Admit(t *testing.T) {
	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			validator := MemcachedValidator{}

			validator.initialized = true
			validator.extClient = extFake.NewSimpleClientset()
			validator.client = fake.NewSimpleClientset()

			objJS, err := meta.MarshalToJson(&c.object, api.SchemeGroupVersion)
			if err != nil {
				panic(err)
			}
			oldObjJS, err := meta.MarshalToJson(&c.oldObject, api.SchemeGroupVersion)
			if err != nil {
				panic(err)
			}

			req := new(admission.AdmissionRequest)

			req.Kind = c.kind
			req.Name = c.objectName
			req.Namespace = c.namespace
			req.Operation = c.operation
			req.UserInfo = c.userInfo
			req.Object.Raw = objJS
			req.OldObject.Raw = oldObjJS

			if c.heatUp {
				if _, err := validator.extClient.KubedbV1alpha1().Memcacheds(c.namespace).Create(&c.object); err != nil && !kerr.IsAlreadyExists(err) {
					t.Errorf(err.Error())
				}
			}
			if c.operation == admission.Delete {
				req.Object = runtime.RawExtension{}
			}
			if c.operation != admission.Update {
				req.OldObject = runtime.RawExtension{}
			}

			response := validator.Admit(req)
			if c.result == true {
				if response.Allowed != true {
					t.Errorf("expected: 'Allowed=true'. but got response: %v", response)
				}
			} else if c.result == false {
				if response.Allowed == true || response.Result.Code == http.StatusInternalServerError {
					t.Errorf("expected: 'Allowed=false', but got response: %v", response)
				}
			}
		})
	}

}

var cases = []struct {
	testName   string
	kind       metaV1.GroupVersionKind
	objectName string
	namespace  string
	operation  admission.Operation
	userInfo   authenticationV1.UserInfo
	object     api.Memcached
	oldObject  api.Memcached
	heatUp     bool
	result     bool
}{
	{"Create Valid Memcached By User",
		requestKind,
		"foo",
		"default",
		admission.Create,
		userIsHooman(),
		sampleMemcached(),
		api.Memcached{},
		false,
		true,
	},
	{"Create Invalid Memcached By User",
		requestKind,
		"foo",
		"default",
		admission.Create,
		userIsHooman(),
		getAwkwardMemcached(),
		api.Memcached{},
		false,
		false,
	},
	{"Create Invalid Memcached By Operator",
		requestKind,
		"foo",
		"default",
		admission.Create,
		userIsHooman(),
		getAwkwardMemcached(),
		api.Memcached{},
		false,
		false,
	},
	{"Edit Memcached Spec.Version By User",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsHooman(),
		editSpecVersion(sampleMemcached()),
		sampleMemcached(),
		false,
		false,
	},
	{"Edit Status By User",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsHooman(),
		editStatus(sampleMemcached()),
		sampleMemcached(),
		false,
		false,
	},
	{"Edit Status By Operator",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsOperator(),
		editStatus(sampleMemcached()),
		sampleMemcached(),
		false,
		true,
	},
	{"Edit Spec.Monitor By User",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsHooman(),
		editSpecMonitor(sampleMemcached()),
		sampleMemcached(),
		false,
		true,
	},
	{"Edit Spec.Monitor By Operator",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsOperator(),
		editSpecMonitor(sampleMemcached()),
		sampleMemcached(),
		false,
		true,
	},
	{"Edit Invalid Spec.Monitor By User",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsHooman(),
		editSpecInvalidMonitor(sampleMemcached()),
		sampleMemcached(),
		false,
		false,
	},
	{"Edit Invalid Spec.Monitor By Operator",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsOperator(),
		editSpecInvalidMonitor(sampleMemcached()),
		sampleMemcached(),
		false,
		false,
	},
	{"Edit Spec.DoNotPause By User",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsHooman(),
		editSpecDoNotPause(sampleMemcached()),
		sampleMemcached(),
		false,
		true,
	},
	{"Delete Memcached when Spec.DoNotPause=true by Operator",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		userIsOperator(),
		sampleMemcached(),
		api.Memcached{},
		true,
		false,
	},
	{"Delete Memcached when Spec.DoNotPause=true by User",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		userIsHooman(),
		sampleMemcached(),
		api.Memcached{},
		true,
		false,
	},
	{"Delete Memcached when Spec.DoNotPause=false by Operator",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		userIsOperator(),
		editSpecDoNotPause(sampleMemcached()),
		api.Memcached{},
		true,
		true,
	},
	{"Delete Memcached when Spec.DoNotPause=false by User",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		userIsHooman(),
		editSpecDoNotPause(sampleMemcached()),
		api.Memcached{},
		true,
		true,
	},
	{"Delete Non Existing Memcached By Operator",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		userIsOperator(),
		api.Memcached{},
		api.Memcached{},
		false,
		true,
	},
	{"Delete Non Existing Memcached By User",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		userIsHooman(),
		api.Memcached{},
		api.Memcached{},
		false,
		true,
	},
}

func sampleMemcached() api.Memcached {
	return api.Memcached{
		TypeMeta: metaV1.TypeMeta{
			Kind:       api.ResourceKindMemcached,
			APIVersion: api.SchemeGroupVersion.String(),
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
			Labels: map[string]string{
				api.LabelDatabaseKind: api.ResourceKindMemcached,
			},
		},
		Spec: api.MemcachedSpec{
			Version:    "1.5.4",
			DoNotPause: true,
		},
	}
}

func getAwkwardMemcached() api.Memcached {
	redis := sampleMemcached()
	redis.Spec.Version = "4.4"
	return redis
}

func editSpecVersion(old api.Memcached) api.Memcached {
	old.Spec.Version = "1.5.3"
	return old
}

func editStatus(old api.Memcached) api.Memcached {
	old.Status = api.MemcachedStatus{
		Phase: api.DatabasePhaseCreating,
	}
	return old
}

func editSpecMonitor(old api.Memcached) api.Memcached {
	old.Spec.Monitor = &kubeMon.AgentSpec{
		Agent: kubeMon.AgentPrometheusBuiltin,
	}
	return old
}

// should be failed because more fields required for COreOS Monitoring
func editSpecInvalidMonitor(old api.Memcached) api.Memcached {
	old.Spec.Monitor = &kubeMon.AgentSpec{
		Agent: kubeMon.AgentCoreOSPrometheus,
	}
	return old
}

func editSpecDoNotPause(old api.Memcached) api.Memcached {
	old.Spec.DoNotPause = false
	return old
}

func userIsOperator() authenticationV1.UserInfo {
	return authenticationV1.UserInfo{
		Username: "system:serviceaccount:kube-system:kubedb-operator",
		Groups: []string{
			"system:serviceaccounts",
			"system:serviceaccounts:kube-system",
			"system:authenticated",
		},
	}
}

func userIsHooman() authenticationV1.UserInfo {
	return authenticationV1.UserInfo{
		Username: "minikube-user",
		Groups: []string{
			"system:masters",
			"system:authenticated",
		},
	}
}
