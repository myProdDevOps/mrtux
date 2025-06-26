package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"mrtux/packages/jenkins"
)

// getAvailableTemplates scans templates directory for .groovy files (excluding _template files)
func getAvailableTemplates() ([]string, error) {
	templatesDir := "templates/jenkins"
	var templates []string

	files, err := os.ReadDir(templatesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read templates directory: %w", err)
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".groovy") && !strings.Contains(file.Name(), "_template") {
			templates = append(templates, file.Name())
		}
	}

	return templates, nil
}

// displayTemplateMenu shows available templates and returns selected template path
func displayTemplateMenu() (string, error) {
	templates, err := getAvailableTemplates()
	if err != nil {
		return "", err
	}

	if len(templates) == 0 {
		return "", fmt.Errorf("no templates found in templates/jenkins directory")
	}

	fmt.Println("\nAvailable templates:")
	for i, template := range templates {
		// Remove .groovy extension for display
		displayName := strings.TrimSuffix(template, ".groovy")
		fmt.Printf("%d. %s\n", i+1, displayName)
	}

	var choice string
	fmt.Printf("Choose template (1-%d): ", len(templates))
	fmt.Scanln(&choice)

	// Parse choice
	choiceNum, err := strconv.Atoi(choice)
	if err != nil || choiceNum < 1 || choiceNum > len(templates) {
		return "", fmt.Errorf("invalid template choice")
	}

	// Return full path
	selectedTemplate := templates[choiceNum-1]
	return filepath.Join("templates/jenkins", selectedTemplate), nil
}

func main() {
	// Load Jenkins configuration
	config, err := jenkins.LoadConfig("packages/jenkins/configs/configs.yaml")
	if err != nil {
		fmt.Printf("❌ Error reading config: %v\n", err)
		os.Exit(1)
	}

	// Create Jenkins client
	client := jenkins.NewClient(config)

	// Create scanner for input
	scanner := bufio.NewScanner(os.Stdin)

	// Display menu
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("🚀 MRTUX JENKINS MANAGER")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("📍 Jenkins URL: %s\n", config.JenkinsURL)
	fmt.Printf("👤 Username: %s\n", config.JenkinsUsername)
	fmt.Println()
	fmt.Println("Action list:")
	fmt.Println("1. ✅ Check Jenkins connection")
	fmt.Println("2. ✅ Add new job")
	fmt.Println("0. ❌ Exit")

	for {
		fmt.Println(strings.Repeat("=", 50))

		var choice string
		fmt.Print("Enter your choice: ")
		fmt.Scanln(&choice)

		switch choice {
		case "1":
			fmt.Println("\n🔄 Testing connection...")
			err := client.TestConnection()
			if err != nil {
				fmt.Printf("❌ Connection failed: %v\n", err)
			} else {
				fmt.Println("✅ Jenkins connection successful!")
				fmt.Printf("🌐 Server: %s\n", client.Config.JenkinsURL)
			}
		case "2":
			fmt.Println("\n📝 Add New Jenkins Job")

			var jobName, customFolder, repoName, branch, helmDeploy string

			// Get job name
			fmt.Print("Enter job name: ")
			fmt.Scanln(&jobName)

			// Get repo name only (not full URL)
			fmt.Print("Enter Git repository name: ")
			scanner.Scan()
			repoName = strings.TrimSpace(scanner.Text())

			// Get custom folder
			fmt.Print("Enter custom folder (default: PROD): ")
			scanner.Scan()
			customFolder = strings.TrimSpace(scanner.Text())
			if customFolder == "" {
				customFolder = "PROD"
			}
			// Build full git URL từ repo name
			fullGitURL := client.BuildGitURL(customFolder, repoName)

			// Get branch (default to main)
			fmt.Print("Enter Git branch (default: main): ")
			scanner.Scan()
			branch = strings.TrimSpace(scanner.Text())
			if branch == "" {
				branch = "main"
			}

			// Get helm deploy name
			fmt.Print("Enter Helm deploy name: ")
			scanner.Scan()
			helmDeploy = strings.TrimSpace(scanner.Text())

			// Display and get template choice
			templatePath, err := displayTemplateMenu()
			if err != nil {
				fmt.Printf("❌ Template selection failed: %v\n", err)
				continue
			}

			// Create job parameters
			params := jenkins.JobParams{
				ProductName: jobName,
				GitRepo:     fullGitURL,
				GitBranch:   branch,
				HelmDeploy:  helmDeploy,
			}

			fmt.Printf("🔄 Creating job '%s' with:\n", jobName)
			fmt.Printf("   📦 Product: %s\n", params.ProductName)
			fmt.Printf("   🔗 Repository: %s\n", params.GitRepo)
			fmt.Printf("   🌿 Branch: %s\n", params.GitBranch)
			fmt.Printf("   ⚓ Helm Deploy: %s\n", params.HelmDeploy)

			result, err := client.AddNewJob(jobName, templatePath, params)
			if err != nil {
				fmt.Printf("❌ Job creation failed: %v\n", err)
			} else {
				fmt.Printf("✅ %s\n", result)
			}
		case "0":
			fmt.Println("👋 Bye!")
			os.Exit(0)
		default:
			fmt.Println("❌ Invalid choice!")
		}
	}
}
