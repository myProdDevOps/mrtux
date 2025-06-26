package jenkins

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// JenkinsConfig represents the configuration for Jenkins connection
type JenkinsConfig struct {
	JenkinsURL      string `yaml:"jenkins_url"`
	GitBaseURL      string `yaml:"git_base_url"`
	JenkinsUsername string `yaml:"jenkins_username"`
	JenkinsPassword string `yaml:"jenkins_password"`
	APIToken        string `yaml:"api_token"`
}

// JenkinsClient represents a Jenkins client
type JenkinsClient struct {
	Config     *JenkinsConfig
	HTTPClient *http.Client
	BaseURL    string
}

// JobParams represents parameters for job creation
type JobParams struct {
	ProductName string
	GitRepo     string
	GitBranch   string
	HelmDeploy  string
}

// LoadConfig loads Jenkins configuration from YAML file
func LoadConfig(configPath string) (*JenkinsConfig, error) {
	data, err := os.ReadFile(configPath) // Read config.yaml file
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config JenkinsConfig
	err = yaml.Unmarshal(data, &config) // Unmarshal yaml data to JenkinsConfig
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

// NewClient creates a new Jenkins client
func NewClient(config *JenkinsConfig) *JenkinsClient {
	// Create HTTP client with timeout and SSL config
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // For self-signed certificates
	}

	httpClient := &http.Client{
		Transport: tr,
		Timeout:   30 * time.Second,
	}

	return &JenkinsClient{
		Config:     config,
		HTTPClient: httpClient,
		BaseURL:    config.JenkinsURL,
	}
}

// getAuthHeader returns the authorization header for Jenkins API as base64 encoded string
func (j *JenkinsClient) getAuthHeader() string {
	// Use API token authentication: username:api_token
	auth := j.Config.JenkinsUsername + ":" + j.Config.APIToken
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
}

// TestConnection tests the connection to Jenkins server
func (j *JenkinsClient) TestConnection() error {
	url := j.BaseURL + "/api/json"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication header
	req.Header.Set("Authorization", j.getAuthHeader())
	req.Header.Set("Content-Type", "application/json")

	resp, err := j.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to Jenkins: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("authentication failed: invalid credentials")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jenkins returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// getCrumb gets Jenkins CSRF protection token for POST requests
func (j *JenkinsClient) getCrumb() (string, string, error) {
	url := j.BaseURL + "/crumbIssuer/api/json"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create crumb request: %w", err)
	}

	req.Header.Set("Authorization", j.getAuthHeader())

	resp, err := j.HTTPClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to get crumb: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Jenkins có thể không có CSRF protection enabled
		return "", "", nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read crumb response: %w", err)
	}

	// Parse crumb từ JSON response đơn giản
	bodyStr := string(body)
	if strings.Contains(bodyStr, "crumb") {
		// Extract crumb value đơn giản
		parts := strings.Split(bodyStr, "\"")
		for i, part := range parts {
			if part == "crumb" && i+2 < len(parts) {
				return "Jenkins-Crumb", parts[i+2], nil
			}
		}
	}

	return "", "", nil
}

// AddNewJob creates a new Jenkins job from pipeline template with parameters
func (j *JenkinsClient) AddNewJob(jobName string, templatePath string, params JobParams) (string, error) {
	// Read pipeline script from template file
	pipelineScript, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to read template file: %w", err)
	}

	// Replace placeholders with actual values
	scriptContent := string(pipelineScript)
	scriptContent = strings.ReplaceAll(scriptContent, "{{PRODUCT_NAME}}", params.ProductName)
	scriptContent = strings.ReplaceAll(scriptContent, "{{GIT_REPO}}", params.GitRepo)
	scriptContent = strings.ReplaceAll(scriptContent, "{{GIT_BRANCH}}", params.GitBranch)
	scriptContent = strings.ReplaceAll(scriptContent, "{{HELM_DEPLOY}}", params.HelmDeploy)

	// Create Jenkins pipeline job XML config with proper format
	jobConfig := fmt.Sprintf(`<?xml version='1.1' encoding='UTF-8'?>
<flow-definition plugin="workflow-job">
  <actions/>
  <description>Pipeline job created by MRTUX tool</description>
  <keepDependencies>false</keepDependencies>
  <properties/>
  <definition class="org.jenkinsci.plugins.workflow.cps.CpsFlowDefinition" plugin="workflow-cps">
    <script><![CDATA[%s]]></script>
    <sandbox>true</sandbox>
  </definition>
  <triggers/>
  <disabled>false</disabled>
</flow-definition>`, scriptContent)

	// Create Jenkins job via API
	url := fmt.Sprintf("%s/createItem?name=%s", j.BaseURL, jobName)

	req, err := http.NewRequest("POST", url, strings.NewReader(jobConfig))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	req.Header.Set("Authorization", j.getAuthHeader())
	req.Header.Set("Content-Type", "application/xml")

	// Get and add CSRF crumb if available
	crumbHeader, crumbValue, err := j.getCrumb()
	if err == nil && crumbHeader != "" {
		req.Header.Set(crumbHeader, crumbValue)
	}

	resp, err := j.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to create job: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusBadRequest {
		return "", fmt.Errorf("job creation failed: job '%s' already exists or invalid config", jobName)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("jenkins returned status %d: %s", resp.StatusCode, string(body))
	}

	return fmt.Sprintf("Job '%s' created successfully", jobName), nil
}

// BuildGitURL constructs full git URL from repo name
func (j *JenkinsClient) BuildGitURL(customFolder string, repoName string) string {
	return fmt.Sprintf("%s/%s/%s.git", j.Config.GitBaseURL, customFolder, repoName)
}
