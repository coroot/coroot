package ch

import (
	"crypto/tls"
	"crypto/x509"
	"os"

	"k8s.io/klog"
)

func TlsConfig(caFile string, insecureSkipVerify bool) *tls.Config {
	cfg := &tls.Config{InsecureSkipVerify: insecureSkipVerify}
	if caFile != "" {
		ca, err := os.ReadFile(caFile)
		if err != nil {
			klog.Fatalln(err)
			return cfg
		}
		pool, err := x509.SystemCertPool()
		if err != nil {
			klog.Warningln("failed to load system cert pool, starting with empty pool:", err)
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(ca) {
			klog.Fatalf("failed to parse CA from %s", caFile)
		}
		cfg.RootCAs = pool
	}
	return cfg
}
