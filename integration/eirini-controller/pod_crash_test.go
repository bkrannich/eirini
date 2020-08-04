package eirini_controller_test

import (
	"context"
	"fmt"
	"time"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/integration/util"
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("PodCrash", func() {
	var (
		config     eirini.Config
		session    *gexec.Session
		lrpGUID    string
		lrpVersion string
		timestamp  time.Time
	)

	BeforeEach(func() {
		config = eirini.Config{
			Properties: eirini.Properties{
				KubeConfig: eirini.KubeConfig{
					Namespace:  fixture.Namespace,
					ConfigPath: fixture.KubeConfigPath,
				},
			},
		}

		session, _ = eiriniBins.EiriniController.Run(config)
		timestamp = time.Now()
	})

	AfterEach(func() {
		session.Kill()
	})

	When("a crashing app is deployed", func() {
		BeforeEach(func() {
			namespace := fixture.Namespace
			lrpGUID = util.GenerateGUID()
			lrpVersion = util.GenerateGUID()

			lrp := &eiriniv1.LRP{
				ObjectMeta: metav1.ObjectMeta{
					Name: "crashing-lrp",
				},
				Spec: eiriniv1.LRPSpec{
					GUID:      lrpGUID,
					Version:   lrpVersion,
					Image:     "busybox",
					AppGUID:   "the-app-guid",
					AppName:   "k-2so",
					SpaceName: "s",
					OrgName:   "o",
					MemoryMB:  256,
					DiskMB:    256,
					CPUWeight: 10,
					Instances: 1,
					AppRoutes: []eiriniv1.Route{{Hostname: "app-hostname-1", Port: 8080}},
					Command:   []string{"sh", "-c", `sleep 0.5; exit 3`},
				},
			}

			_, err := fixture.EiriniClientset.
				EiriniV1().
				LRPs(namespace).
				Create(context.Background(), lrp, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates crash events", func() {
			eventsClient := fixture.Clientset.CoreV1().Events(fixture.Namespace)
			var events []corev1.Event
			getEvents := func() int {
				eventList, err := eventsClient.List(context.Background(), metav1.ListOptions{
					FieldSelector: "involvedObject.kind=LRP",
				})
				Expect(err).NotTo(HaveOccurred())
				events = eventList.Items
				return len(events)
			}
			Eventually(getEvents, "20s").Should(BeNumerically(">", 0))

			crash := events[0]
			Expect(crash.Name).To(HavePrefix("k-2so"))
			Expect(crash.Type).To(Equal("Warning"))
			Expect(crash.Count).To(BeNumerically("==", 1))
			Expect(crash.FirstTimestamp.Time).To(BeTemporally(">", timestamp))
			Expect(crash.LastTimestamp.Time).To(BeTemporally("==", crash.FirstTimestamp.Time))
			Expect(crash.EventTime.Time).To(BeTemporally(">", crash.LastTimestamp.Time))
			Expect(crash.Reason).To(Equal("Container: Error"))
			Expect(crash.Message).To(Equal("Container terminated with exit code: 3"))
			Expect(crash.Source.Component).To(Equal("eirini-controller"))
			Expect(crash.Labels).To(HaveKeyWithValue("cloudfoundry.org/instance_index", "0"))
			Expect(crash.Annotations).To(HaveKeyWithValue("cloudfoundry.org/process_guid", fmt.Sprintf("%s-%s", lrpGUID, lrpVersion)))
			Expect(crash.InvolvedObject.Kind).To(Equal("LRP"))
			Expect(crash.InvolvedObject.Name).To(Equal("crashing-lrp"))
			Expect(crash.InvolvedObject.Namespace).To(Equal(fixture.Namespace))
		})

		It("creates crash loop backoff events", func() {
			eventsClient := fixture.Clientset.CoreV1().Events(fixture.Namespace)
			getEvents := func() []corev1.Event {
				eventList, err := eventsClient.List(context.Background(), metav1.ListOptions{
					FieldSelector: "involvedObject.kind=LRP",
				})
				Expect(err).NotTo(HaveOccurred())
				return eventList.Items
			}
			Eventually(getEvents, "20s").Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Reason": Equal("Container: CrashLoopBackOff"),
			})))
		})

		It("updates crash events", func() {
			eventsClient := fixture.Clientset.CoreV1().Events(fixture.Namespace)
			getEvents := func() []corev1.Event {
				eventList, err := eventsClient.List(context.Background(), metav1.ListOptions{
					FieldSelector: "involvedObject.kind=LRP",
				})
				Expect(err).NotTo(HaveOccurred())
				return eventList.Items
			}
			Eventually(getEvents, "20s").Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Reason": HavePrefix("Container: Error"),
				"Count":  BeNumerically(">", 1),
			})))
		})
	})
})
