package stset

import (
	"strings"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s/utils"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/eirini/util"
	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
)

//counterfeiter:generate . PodGetter
//counterfeiter:generate . EventGetter

const (
	eventKilling          = "Killing"
	eventFailedScheduling = "FailedScheduling"
	eventFailedScaleUp    = "NotTriggerScaleUp"
)

type PodGetter interface {
	GetByLRPIdentifier(opi.LRPIdentifier) ([]corev1.Pod, error)
}

type EventGetter interface {
	GetByPod(pod corev1.Pod) ([]corev1.Event, error)
}

type Getter struct {
	logger         lager.Logger
	podGetter      PodGetter
	eventGetter    EventGetter
	lrpMapper      StatefulSetToLRP
	getStatefulSet getStatefulSetFunc
}

func NewGetter(
	logger lager.Logger,
	statefulSetGetter StatefulSetByLRPIdentifierGetter,
	podGetter PodGetter,
	eventGetter EventGetter,
	lrpMapper StatefulSetToLRP,
) Getter {
	return Getter{
		logger:         logger,
		podGetter:      podGetter,
		eventGetter:    eventGetter,
		lrpMapper:      lrpMapper,
		getStatefulSet: newGetStatefulSetFunc(statefulSetGetter),
	}
}

func (g *Getter) Get(identifier opi.LRPIdentifier) (*opi.LRP, error) {
	logger := g.logger.Session("get", lager.Data{"guid": identifier.GUID, "version": identifier.Version})

	return g.getLRP(logger, identifier)
}

func (g *Getter) GetInstances(identifier opi.LRPIdentifier) ([]*opi.Instance, error) {
	logger := g.logger.Session("get-instance", lager.Data{"guid": identifier.GUID, "version": identifier.Version})
	if _, err := g.getLRP(logger, identifier); errors.Is(err, eirini.ErrNotFound) {
		return nil, err
	}

	pods, err := g.podGetter.GetByLRPIdentifier(identifier)
	if err != nil {
		logger.Error("failed-to-list-pods", err)

		return nil, errors.Wrap(err, "failed to list pods")
	}

	instances := []*opi.Instance{}

	for _, pod := range pods {
		events, err := g.eventGetter.GetByPod(pod)
		if err != nil {
			logger.Error("failed-to-get-events", err)

			return nil, errors.Wrapf(err, "failed to get events for pod %s", pod.Name)
		}

		if isStopped(events) {
			continue
		}

		index, err := util.ParseAppIndex(pod.Name)
		if err != nil {
			logger.Error("failed-to-parse-app-index", err)

			return nil, errors.Wrap(err, "failed to parse pod index")
		}

		since := int64(0)
		if pod.Status.StartTime != nil {
			since = pod.Status.StartTime.UnixNano()
		}

		var state, placementError string
		if hasInsufficientMemory(events) {
			state, placementError = opi.ErrorState, opi.InsufficientMemoryError
		} else {
			state = utils.GetPodState(pod)
		}

		instance := opi.Instance{
			Since:          since,
			Index:          index,
			State:          state,
			PlacementError: placementError,
		}
		instances = append(instances, &instance)
	}

	return instances, nil
}

func (g *Getter) getLRP(logger lager.Logger, identifier opi.LRPIdentifier) (*opi.LRP, error) {
	statefulset, err := g.getStatefulSet(identifier)
	if err != nil {
		logger.Error("failed-to-get-statefulset", err)

		return nil, err
	}

	lrp, err := g.lrpMapper(*statefulset)
	if err != nil {
		logger.Error("failed-to-map-statefulset-to-lrp", err)

		return nil, err
	}

	return lrp, nil
}

func isStopped(events []corev1.Event) bool {
	if len(events) == 0 {
		return false
	}

	event := events[len(events)-1]

	return event.Reason == eventKilling
}

func hasInsufficientMemory(events []corev1.Event) bool {
	if len(events) == 0 {
		return false
	}

	event := events[len(events)-1]

	return (event.Reason == eventFailedScheduling || event.Reason == eventFailedScaleUp) &&
		strings.Contains(event.Message, "Insufficient memory")
}
