package main

import (
	"crypto/sha256"
	"flag"
	"io/ioutil"
	"os"

	hook "github.com/hiraken-w/mutating-webhook-sidecar-injector/webhook"

	"github.com/golang/glog"
	"gopkg.in/yaml.v2"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var log = logf.Log.WithName("example-controller")

type HookParamters struct {
	certDir       string
	sidecarConfig string
	port          int
}

func loadConfig(confgFile string) (*hook.Config, error) {
	data, err := ioutil.ReadFile(confgFile)
	if err != nil {
		return nil, err
	}
	glog.Info("New configuration: sha256sum %x", sha256.Sum256(data))

	var cfg hook.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func main() {
	var params HookParamters

	flag.IntVar(&params.port, "port", 8443, "Webhook port")
	flag.StringVar(&params.certDir, "certDir", "/certs/", "Webhook certificate folder")
	flag.StringVar(&params.sidecarConfig, "sidecarConfig", "/etc/webhook/config/sidecarconfig.yaml", "Webhook sidecar config")
	flag.Parse()

	logf.SetLogger(zap.New(zap.UseDevMode(true)))

	entryLog := log.WithName("entrypoint")

	entryLog.Info("setting up manager")
	// create a new controller manager
	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		entryLog.Error(err, "unable to set up overall controller manager")
		os.Exit(1)
	}
	config, err := loadConfig(params.sidecarConfig)

	entryLog.Info("setting up webhook server")
	hookServer := mgr.GetWebhookServer()
	hookServer.Port = params.port
	hookServer.CertDir = params.certDir

	entryLog.Info("registering webhooks to the webhook server")
	hookServer.Register("/mutate", &webhook.Admission{
		Handler: &hook.SidecarInjector{Name: "Logger", Client: mgr.GetClient(), SidecarConfig: config}})

	entryLog.Info("starting manager")
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		entryLog.Error(err, "unable to run manager")
		os.Exit(1)
	}
}
