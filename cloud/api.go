package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/coroot/coroot/db"
)

const (
	settingName = "cloud_integration_settings"
)

var (
	URL = "https://coroot.com"
)

func init() {
	// for dev
	if u := os.Getenv("CLOUD_URL"); u != "" {
		URL = u
	}
}

type Settings struct {
	ApiKey string      `json:"api_key" yaml:"apiKey"`
	RCA    SettingsRCA `json:"rca" yaml:"rca"`
}

type SettingsRCA struct {
	DisableIncidentsAutoInvestigation bool `json:"disable_incidents_auto_investigation" yaml:"disableIncidentsAutoInvestigation"`
}

func (s *Settings) Validate() error {
	if s.ApiKey == "" {
		return errors.New("api key is required")
	}
	return nil
}

type Api struct {
	db             *db.DB
	deploymentUuid string
	instanceUuid   string
	returnUrl      string
}

func API(db *db.DB, deploymentUuid, instanceUuid, returnUrl string) *Api {
	return &Api{db: db, deploymentUuid: deploymentUuid, instanceUuid: instanceUuid, returnUrl: returnUrl}
}

func (api *Api) GetSettings() (Settings, error) {
	var settings Settings
	err := api.db.GetSetting(settingName, &settings)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return settings, nil
		}
		return settings, err
	}
	return settings, nil
}

func (api *Api) SaveSettings(settings Settings) error {
	return api.db.SetSetting(settingName, settings)
}

func (api *Api) request(ctx context.Context, method, path, contentType, contentEncoding string, body io.Reader, dest any) error {
	settings, err := api.GetSettings()
	if err != nil {
		return err
	}
	if settings.ApiKey == "" {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, method, URL+"/api"+path, body)
	if err != nil {
		return err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if contentEncoding != "" {
		req.Header.Set("Content-Encoding", contentEncoding)
	}
	req.Header.Set("X-API-KEY", settings.ApiKey)
	req.Header.Set("X-DEPLOYMENT-UUID", api.deploymentUuid)
	req.Header.Set("X-INSTANCE-UUID", api.instanceUuid)
	req.Header.Set("X-RETURN-URL", api.returnUrl)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		msg, _ := io.ReadAll(resp.Body)
		msg = bytes.TrimSpace(msg)
		if len(msg) == 0 {
			msg = []byte(resp.Status)
		}
		return fmt.Errorf("request failed %d: %s", resp.StatusCode, string(msg))
	}
	if err = json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	return nil
}
