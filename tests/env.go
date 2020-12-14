package tests

import (
	"os"
	"path/filepath"

	// nolint:golint,stylecheck
	. "github.com/onsi/ginkgo"

	// nolint:golint,stylecheck
	. "github.com/onsi/gomega"
)

const DefaultApplicationServiceAccount = "eirini"

func GetKubeconfig() string {
	kubeconfPath := os.Getenv("INTEGRATION_KUBECONFIG")
	if kubeconfPath != "" {
		return kubeconfPath
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		Fail("INTEGRATION_KUBECONFIG not provided, failed to use default: " + err.Error())
	}

	kubeconfPath = filepath.Join(homeDir, ".kube", "config")

	_, err = os.Stat(kubeconfPath)
	if os.IsNotExist(err) {
		return ""
	}

	Expect(err).NotTo(HaveOccurred())

	return kubeconfPath
}

func GetEiriniDockerHubPassword() string {
	return lookupOptionalEnv("EIRINIUSER_PASSWORD")
}

func GetApplicationServiceAccount() string {
	serviceAccountName := os.Getenv("APPLICATION_SERVICE_ACCOUNT")
	if serviceAccountName != "" {
		return serviceAccountName
	}

	return DefaultApplicationServiceAccount
}

func GetEiriniSystemNamespace() string {
	return lookupOptionalEnv("EIRINI_SYSTEM_NS")
}

func GetEiriniWorkloadsNamespace() string {
	return lookupOptionalEnv("EIRINI_WORKLOADS_NS")
}

func getEiriniTLSSecretName() string {
	return lookupOptionalEnv("EIRINI_TLS_SECRET")
}

func GetEiriniAddress() string {
	return lookupOptionalEnv("EIRINI_ADDRESS")
}

func lookupOptionalEnv(key string) string {
	value, set := os.LookupEnv(key)
	if !set || value == "" {
		Skip("Please export optional environment variable " + key + " to run this test")
	}

	return value
}
