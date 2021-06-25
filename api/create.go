package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/cloudflare/cloudflare-go"
)

var api *cloudflare.API
var accountID, zoneID, projectDir, architecture string

func init() {
	// Construct a new API object
	var err error
	var ok bool
	if projectDir, ok = os.LookupEnv("SPINUP_PROJECT_DIR"); !ok {
		log.Fatalf("FATAL: getting environment variable SPINUP_PROJECT_DIR")
	}
	// MYSTERY: there is api.accountID but its always empty. Need to figure why. Until then we are explicity passing account id
	if accountID, ok = os.LookupEnv("CF_ACCOUNT_ID"); !ok {
		log.Fatalf("FATAL: getting environment variable CF_ACCOUNT_ID")
	}
	if zoneID, ok = os.LookupEnv("CF_ZONE_ID"); !ok {
		log.Fatalf("FATAL: getting environment variable CF_ZONE_ID")
	}
	api, err = cloudflare.New(os.Getenv("CF_API_KEY"), os.Getenv("CF_API_EMAIL"))
	if err != nil {
		log.Fatalf("FATAL: creating new cloudflare client %v", err)
	}
	if architecture, ok = os.LookupEnv("ARCHITECTURE"); !ok {
		log.Fatalf("FATAL: getting environment variable ARCHITECTURE")
	}
	// USEFUL: uncomment in case if you need to delete a tunnel. Replace with tunnel UUID
	/* if err := api.DeleteArgoTunnel(context.Background(), accountID, "replace-me-with-uuid-ofyourtunnel"); err != nil {
		log.Print("ERROR: deleting argo tunnel 1b8f0228-7552-4329-be22-4f74540c673b")
	} */
	log.Println("INFO: successfully created a new cloudflare client")
}

type Service struct {
	name     string
	duration time.Duration
	resource map[string]interface{}
	tunnel   cloudflare.ArgoTunnel
	userID   string
	// eg. arm64v8 or arm32v7
	architecture string
	port         uint
}

type tunnelConfig struct {
	accountTag   string
	tunnelSecret string
	tunnelID     string
	tunnelName   string
}

func Hello(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "hello !! Welcome to spinup \n")
}

func CreateService(w http.ResponseWriter, req *http.Request) {
	ctx := context.Background()
	if req.Method != "POST" {
		http.Error(w, "Method is not supported.", http.StatusNotFound)
		return
	}
	var c Service
	byteArray, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Fatalf("fatal: reading from readall body %v", req.Body)
	}
	err = json.Unmarshal(byteArray, &c)
	if err != nil {
		log.Fatalf("fatal: reading from readall body %v", req.Body)
	}
	if c.name != "postgres" {
		fmt.Fprintf(w, "currently we don't support %s", c.name)
		return
	}
	c.port = 5432
	c.tunnel.Secret = randSeq(32)
	c.tunnel.ID, err = createTunnel(accountID, c.userID, c.tunnel.Secret)
	if err != nil {
		log.Printf("ERROR: creating argo tunnel %v", err)
		http.Error(w, "Error creating tunnel", 500)
		return
	}
	rr := cloudflare.DNSRecord{
		Type:    "CNAME",
		Name:    c.userID,
		Content: c.tunnel.ID + ".cfargotunnel.com",
	}
	if _, err = createCNAME(ctx, rr); err != nil {
		log.Printf("ERROR: creating cloudflare CNAME for %s %v", rr.Name, err)
		http.Error(w, "Error creating cloudflare CNAME", 500)
		return
	}
	tc := tunnelConfig{
		accountTag:   accountID,
		tunnelSecret: c.tunnel.Secret,
		tunnelID:     c.tunnel.ID,
		tunnelName:   c.userID,
	}
	if err = prepareService(c, tc); err != nil {
		log.Printf("ERROR: preparing service for %s %v", c.userID, err)
		http.Error(w, "Error preparing service", 500)
	}
	return
}

func createTunnel(accountID, name, secret string) (string, error) {
	ctx := context.Background()
	cftunnel, err := api.CreateArgoTunnel(ctx, accountID, name, secret)
	if err != nil {
		return "", err
	}
	log.Printf("INFO: argo tunnel created ID: %s ", cftunnel.ID)
	return cftunnel.ID, nil
}

func listTunnels(accountID string) error {
	ctx := context.Background()
	tunnels, err := api.ArgoTunnels(ctx, accountID)
	if err != nil {
		log.Printf("ERROR: listing cloudflare tunnels %v", err)
	}
	fmt.Printf("%+v\n", tunnels)
	return nil
}

func createCNAME(ctx context.Context, rr cloudflare.DNSRecord) (string, error) {
	res, err := api.CreateDNSRecord(ctx, zoneID, rr)
	if err != nil {

		return "", err
	}
	log.Printf("INFO: cloudflare CNAME created for %s ", rr.Name)
	return res.Result.ID, nil
}

func prepareService(s Service, tc tunnelConfig) error {
	err := os.Mkdir(projectDir+"/"+s.userID, 0755)
	if err != nil {
		return fmt.Errorf("ERROR: creating project directory at %s", projectDir+"/"+s.userID)
	}
	if err := createJSONFile(projectDir+"/"+s.userID+"/"+tc.tunnelID+".json", tc); err != nil {
		return fmt.Errorf("ERROR: creating tunnel json file %v", err)
	}
	if err := createDockerComposeFile(projectDir+"/"+s.userID+"/", s); err != nil {
		return fmt.Errorf("ERROR: creating service docker-compose file %v", err)
	}
	if err := createDockerfile(projectDir+"/"+s.userID+"/", s); err != nil {
		return fmt.Errorf("ERROR: creating service docker file %v", err)
	}
	if err := createConfigfile(projectDir+"/"+s.userID+"/", s); err != nil {
		return fmt.Errorf("ERROR: creating service config file %v", err)
	}
	return nil
}
