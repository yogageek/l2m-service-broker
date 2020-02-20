package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path"
	"strconv"
	"syscall"

	"github.com/golang/glog"
	"github.com/pmorie/osb-broker-lib/pkg/metrics"
	"github.com/pmorie/osb-broker-lib/pkg/rest"
	"github.com/pmorie/osb-broker-lib/pkg/server"
	prom "github.com/prometheus/client_golang/prometheus"
	clientset "k8s.io/client-go/kubernetes"
	clientrest "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"advgitlab.eastasia.cloudapp.azure.com/pcf-resource/wise-paas-service-manager-mongodb/pkg/broker"
	"advgitlab.eastasia.cloudapp.azure.com/pcf-resource/wise-paas-service-manager-mongodb/pkg/debg"
	"advgitlab.eastasia.cloudapp.azure.com/pcf-resource/wise-paas-service-manager-mongodb/pkg/middleware"
	"advgitlab.eastasia.cloudapp.azure.com/pcf-resource/wise-paas-service-manager-mongodb/pkg/recorder"
)

var options struct {
	broker.Options

	Port                 int
	Insecure             bool
	TLSCert              string
	TLSKey               string
	TLSCertFile          string
	TLSKeyFile           string
	AuthenticateK8SToken bool
	KubeConfig           string
}

func init() {
	flag.IntVar(&options.Port, "port", 8000, "use '--port' option to specify the port for broker to listen on")
	flag.BoolVar(&options.Insecure, "insecure", false, "use --insecure to use HTTP vs HTTPS.")
	flag.StringVar(&options.TLSCertFile, "tls-cert-file", "", "File containing the default x509 Certificate for HTTPS. (CA cert, if any, concatenated after server cert).")
	flag.StringVar(&options.TLSKeyFile, "tls-private-key-file", "", "File containing the default x509 private key matching --tls-cert-file.")
	flag.StringVar(&options.TLSCert, "tlsCert", "", "base-64 encoded PEM block to use as the certificate for TLS. If '--tlsCert' is used, then '--tlsKey' must also be used.")
	flag.StringVar(&options.TLSKey, "tlsKey", "", "base-64 encoded PEM block to use as the private key matching the TLS certificate.")
	flag.BoolVar(&options.AuthenticateK8SToken, "authenticate-k8s-token", false, "option to specify if the broker should validate the bearer auth token with kubernetes")
	flag.StringVar(&options.KubeConfig, "kube-config", "", "specify the kube config path to be used")
	broker.AddFlags(&options.Options)
	flag.Parse() //首先用flag.Parse解析參數
}

func main() {
	fmt.Println("flag.Args():", flag.Args())
	// //測試輸入了幾個args
	for i := 0; i < len(os.Args); i++ {
		fmt.Println("os.Args[", i, "]:", os.Args[i])
		// 	// go run main.go run-broker
		// 	// 参数0: c:\gotool\src\deploy\src\github.com\starkandwayne\kafka-service-broker\cmd\broker\__debug_bin
		// 	// 参数1: run-broker
	}
	fmt.Println("OPS_PG_HOST1:", os.Getenv("OPS_PG_HOST1"))
	fmt.Println("OPS_PG_PORT1:", os.Getenv("OPS_PG_PORT1"))
	fmt.Println("MAX_INSTANCE_PER_DB:", os.Getenv("MAX_INSTANCE_PER_DB"))
	fmt.Println("MAX_BIND_PER_INSTANCE:", os.Getenv("MAX_BIND_PER_INSTANCE"))

	if err := run(); err != nil && err != context.Canceled && err != context.DeadlineExceeded {
		glog.Fatalln(err)
	}
}

func run() error {
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	go cancelOnInterrupt(ctx, cancelFunc)
	// go cronjob.New().Run()
	return runWithContext(ctx)
}

func runWithContext(ctx context.Context) error {
	//  (not using)
	//目前第一個參數都是檔案路徑，不知何種情況下第一個參數會是version
	if flag.Arg(0) == "version" {
		fmt.Printf("%s/%s\n", path.Base(os.Args[0]), "0.1.2")
		return nil
	}
	//strconv.Itoa = int to string
	addr := ":" + strconv.Itoa(options.Port)

	b, err := broker.NewService(options.Options)
	if err != nil {
		return err
	}
	// fmt.Println("broker.NewKafkaService(options.Options):", b)

	// Prom. metrics (not using)
	reg := prom.NewRegistry()
	osbMetrics := metrics.New()
	reg.MustRegister(osbMetrics)

	api, err := rest.NewAPISurface(b, osbMetrics)
	if err != nil {
		return err
	}
	s := server.New(api, reg)

	//remove k8s token
	s.Router.Use(middleware.BasicAuth)

	//會檢查DB是否已經存在，目前DB不存在
	// initiate checking database for recorder
	if err := recorder.CheckConnection(); err != nil {
		glog.Error(err)
	}
	//如果操作紀錄DB's Tables不存在就創建(維運人員會先建立DB)
	if err := recorder.InitRecorder(); err != nil {
		glog.Error(err)
	}

	glog.V(2).Infof("Starting Broker!")
	fmt.Println("Starting Broker!")

	//just run
	err = s.Run(ctx, addr)
	if err != nil {
		fmt.Println("err:", err)
	}
	return nil
}

// no use
func getKubernetesClient(kubeConfigPath string) (clientset.Interface, error) {
	var clientConfig *clientrest.Config
	var err error
	if kubeConfigPath == "" {
		clientConfig, err = clientrest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	} else {
		config, err := clientcmd.LoadFromFile(kubeConfigPath)
		if err != nil {
			return nil, err
		}

		clientConfig, err = clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{}).ClientConfig()
		if err != nil {
			return nil, err
		}
	}
	return clientset.NewForConfig(clientConfig)
}

//no use
func cancelOnInterrupt(ctx context.Context, f context.CancelFunc) {
	term := make(chan os.Signal)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-term:
			debg.RemovEnv()
			glog.Infof("Received SIGTERM, exiting gracefully...")
			f()
			os.Exit(0)
		case <-ctx.Done():
			os.Exit(0)
		}
	}
}
