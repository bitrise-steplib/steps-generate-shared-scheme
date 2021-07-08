package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bitrise-io/go-steputils/stepconf"
	"github.com/bitrise-io/go-utils/colorstring"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-xcode/xcodeproject/xcscheme"
)

// Input ...
type Input struct {
	ProjectPath string `env:"project_path,file"`
}

func failf(format string, v ...interface{}) {
	log.Errorf(format, v...)
	os.Exit(1)
}

func main() {
	var config Input
	if err := stepconf.Parse(&config); err != nil {
		failf("Invalid Input: %v", err)
	}

	stepconf.Print(config)
	fmt.Println()

	containerPath, err := pathutil.AbsPath(config.ProjectPath)
	if err != nil {
		failf("Failed to convert Project or Workspace path to absolute path: %v", err)
	}

	container, err := openContainer(containerPath)
	if err != nil {
		failf("Error: %v", err)
	}

	log.Infof("Collecting existing Schemes...")
	containerToSchemes, err := container.schemes()
	if err != nil {
		failf("Could not list Schemes: %v", err)
	}

	log.Donef("Schemes:")
	numSharedSchemes := printSchemes(true, containerToSchemes, containerPath)

	if numSharedSchemes > 0 {
		fmt.Println()
		log.Donef("Found %d shared Scheme(s)", numSharedSchemes)
		os.Exit(0)
	}

	// Generate schemes
	fmt.Println()
	log.Errorf("No shared Schemes found...")
	log.Errorf("The newly generated Schemes may differ from the ones in your Project.")
	log.Errorf("Make sure to share your Schemes, to prevent unexpected behaviour.")

	fmt.Println()
	log.Infof("Generating Schemes")

	projects, missingProjects, err := container.projects()
	if err != nil {
		failf("Error: %v", err)
	}

	for _, missingProject := range missingProjects {
		log.Warnf("Skipping Project (%s), as it is not present", pathRelativeToWorkspace(missingProject, containerPath))
	}

	for _, project := range projects {
		log.Printf("Recreating Schemes for: %s", filepath.Base(project.Path))
		schemes := project.ReCreateSchemes()

		for _, scheme := range schemes {
			if err := project.SaveSharedScheme(scheme); err != nil {
				failf("Failed to save Scheme: %v", err)
			}
		}
	}

	container, err = openContainer(containerPath)
	if err != nil {
		failf("Error: %v", err)
	}
	containerToSchemesNew, err := container.schemes()
	if err != nil {
		failf("Could not list Schemes: %v", err)
	}

	fmt.Println()
	log.Infof("Created Schemes:")
	numGenSharedSchemes := printSchemes(false, containerToSchemesNew, containerPath)

	fmt.Println()
	if numGenSharedSchemes > 0 {
		log.Donef("Number of new shared Schemes: %d", numGenSharedSchemes)
	} else {
		log.Warnf("No new Schemes created.")
	}
}

func pathRelativeToWorkspace(project, workspace string) string {
	parentDir, _ := filepath.Split(workspace)
	relPath, err := filepath.Rel(filepath.Join(parentDir), project)
	if err != nil {
		log.Warnf("%s", err)
		return project
	}

	return relPath
}

func printSchemes(includeUserSchemes bool, containerToSchemes map[string][]xcscheme.Scheme, containerPath string) int {
	var sharedSchemes []xcscheme.Scheme
	for container, schemes := range containerToSchemes {
		log.Printf("- %s", pathRelativeToWorkspace(container, containerPath))
		for _, scheme := range schemes {
			if scheme.IsShared {
				sharedSchemes = append(sharedSchemes, scheme)
				log.Printf("  - %s (Shared)", scheme.Name)
			} else if includeUserSchemes {
				log.Printf(colorstring.Yellow(fmt.Sprintf("  - %s (User)", scheme.Name)))
			}
		}
	}

	return len(sharedSchemes)
}
