package stset

import (
	"fmt"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/opi"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
)

//counterfeiter:generate . StatefulSetByLRPIdentifierGetter

type StatefulSetByLRPIdentifierGetter interface {
	GetByLRPIdentifier(id opi.LRPIdentifier) ([]appsv1.StatefulSet, error)
}

type getStatefulSetFunc func(identifier opi.LRPIdentifier) (*appsv1.StatefulSet, error)

func newGetStatefulSetFunc(stSetGetter StatefulSetByLRPIdentifierGetter) getStatefulSetFunc {
	return func(identifier opi.LRPIdentifier) (*appsv1.StatefulSet, error) {
		statefulSets, err := stSetGetter.GetByLRPIdentifier(identifier)
		if err != nil {
			return nil, errors.Wrap(err, "failed to list statefulsets")
		}

		switch len(statefulSets) {
		case 0:
			return nil, eirini.ErrNotFound
		case 1:
			return &statefulSets[0], nil
		default:
			return nil, fmt.Errorf("multiple statefulsets found for LRP identifier %+v", identifier)
		}
	}
}
