package main

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/eirini"
	cmdcommons "code.cloudfoundry.org/eirini/cmd"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/kubelet"
	"code.cloudfoundry.org/eirini/metrics"
	"code.cloudfoundry.org/eirini/util"
	"code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/lager"
	"github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

type options struct {
	ConfigFile string `short:"c" long:"config" description:"Config for running metrics-collector"`
}

func main() {
	var opts options
	_, err := flags.ParseArgs(&opts, os.Args)
	cmdcommons.ExitIfError(err)

	cfg, err := readMetricsCollectorConfigFromFile(opts.ConfigFile)
	cmdcommons.ExitIfError(err)

	clientset := cmdcommons.CreateKubeClient(cfg.ConfigPath)
	metricsClient := cmdcommons.CreateMetricsClient(cfg.ConfigPath)

	verifyCertFilesExist(cfg)
	tlsConfig, err := loggregator.NewIngressTLSConfig(
		cmdcommons.GetOrDefault(cfg.LoggregatorCAPath, eirini.LoggregatorCAPath),
		cmdcommons.GetOrDefault(cfg.LoggregatorCertPath, eirini.LoggregatorCrtPath),
		cmdcommons.GetOrDefault(cfg.LoggregatorKeyPath, eirini.LoggregatorKeyPath),
	)
	cmdcommons.ExitfIfError(err, "Failed to create loggregator tls config")

	loggregatorClient, err := loggregator.NewIngressClient(
		tlsConfig,
		loggregator.WithAddr(cfg.LoggregatorAddress),
		loggregator.WithLogger(log.New(os.Stdout, "loggregator-ingress-client", log.LstdFlags)),
	)
	cmdcommons.ExitIfError(err)

	defer func() {
		err = loggregatorClient.CloseSend()
		cmdcommons.ExitIfError(err)
	}()

	launchMetricsEmitter(
		clientset,
		metricsClient,
		loggregatorClient,
		cfg.Namespace,
		cfg.AppMetricsEmissionIntervalInSecs,
	)
}

func launchMetricsEmitter(
	clientset kubernetes.Interface,
	metricsClient metricsclientset.Interface,
	loggregatorClient metrics.LoggregatorClient,
	namespace string,
	metricsEmissionInterval int,
) {
	podClient := clientset.CoreV1().Pods(namespace)

	podMetricsClient := metricsClient.MetricsV1beta1().PodMetricses(namespace)
	metricsLogger := lager.NewLogger("metrics")
	metricsLogger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	tickerInterval := eirini.AppMetricsEmissionIntervalInSecs
	if metricsEmissionInterval > 0 {
		tickerInterval = metricsEmissionInterval
	}

	collectorScheduler := &util.TickerTaskScheduler{
		Ticker: time.NewTicker(time.Duration(tickerInterval) * time.Second),
		Logger: metricsLogger.Session("collector.scheduler"),
	}

	metricsCollectorLogger := metricsLogger.Session("metrics-collector", lager.Data{})
	diskClientLogger := metricsCollectorLogger.Session("disk-metrics-client", lager.Data{})
	kubeletClient := kubelet.NewClient(clientset.CoreV1().RESTClient())
	diskClient := kubelet.NewDiskMetricsClient(clientset.CoreV1().Nodes(),
		kubeletClient,
		namespace,
		diskClientLogger)
	collector := k8s.NewMetricsCollector(podMetricsClient, podClient, diskClient, metricsCollectorLogger)

	emitter := metrics.NewLoggregatorEmitter(loggregatorClient)

	collectorScheduler.Schedule(func() error {
		return k8s.ForwardMetricsToEmitter(collector, emitter)
	})
}

func readMetricsCollectorConfigFromFile(path string) (*eirini.MetricsCollectorConfig, error) {
	fileBytes, err := ioutil.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read file")
	}

	var conf eirini.MetricsCollectorConfig
	err = yaml.Unmarshal(fileBytes, &conf)

	return &conf, errors.Wrap(err, "failed to unmarshal yaml")
}

func verifyCertFilesExist(cfg *eirini.MetricsCollectorConfig) {
	cmdcommons.VerifyFileExists(cfg.LoggregatorCAPath, "Loggregator CA")
	cmdcommons.VerifyFileExists(cfg.LoggregatorCertPath, "Loggregator Cert")
	cmdcommons.VerifyFileExists(cfg.LoggregatorKeyPath, "Loggregator Key")
}
