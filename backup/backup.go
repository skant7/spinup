package backup

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/spinup-host/api"
	"github.com/spinup-host/templates"
)

func TriggerBackup() {

}
func Schedule() {
	log.Println("INFO: starting backup")
	api.CreateDockerComposeFile()
	outputPath := filepath.Join(absolutepath, "docker-compose.yml")
	// Create the file:
	f, err := os.Create(outputPath)
	if err != nil {
		panic(err)
	}

	defer f.Close() // don't forget to close the file when finished.
	templ, err := template.ParseFS(templates.DockerTempl, "templates/docker-compose-template.yml")
	if err != nil {
		return fmt.Errorf("ERROR: parsing template file %v", err)
	}
	// TODO: not sure is there a better way to pass data to template
	// A lot of this data is redundant. Already available in Service struct
	data := struct {
		UserID       string
		Architecture string
		Type         string
		Port         int
		Secret       string
	}{
		s.UserID,
		s.Architecture,
		s.Db.Type,
		s.Db.Port,
		"replaceme",
	}
	err = templ.Execute(f, data)
	if err != nil {
		return fmt.Errorf("ERROR: executing template file %v", err)
	}
	return nil
}
