package utils

import (
	"fmt"
	"k8s.io/klog"
	"os"
)

func CreateDirectoryIfNotExists(path string) error {
	st, err := os.Stat(path)
	if err == nil {
		if st.IsDir() {
			return nil
		}
		return fmt.Errorf("path %s is not directory", path)
	}
	if !os.IsNotExist(err) {
		return err
	}
	klog.Infof("creating dir", path)
	if err := os.Mkdir(path, 0755); err != nil {
		return fmt.Errorf("failed to create dir %s: %w", path, err)
	}
	return nil
}
