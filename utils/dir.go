package utils

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/google/uuid"
	"k8s.io/klog"
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
	klog.Infof("creating dir %s", path)
	if err := os.Mkdir(path, 0755); err != nil {
		return fmt.Errorf("failed to create dir %s: %w", path, err)
	}
	return nil
}

func GetInstanceUuid(dataDir string) string {
	instanceUuid := ""
	filePath := path.Join(dataDir, "instance.uuid")
	data, err := os.ReadFile(filePath)
	if err != nil && !os.IsNotExist(err) {
		klog.Errorln("failed to read instance id:", err)
	}
	instanceUuid = strings.TrimSpace(string(data))
	if _, err = uuid.Parse(instanceUuid); err != nil {
		instanceUuid = uuid.NewString()
		if err = os.WriteFile(filePath, []byte(instanceUuid), 0644); err != nil {
			klog.Errorln("failed to write instance id:", err)
			return ""
		}
	}
	return instanceUuid
}
