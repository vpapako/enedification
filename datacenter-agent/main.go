package main

import (
	"bytes"
	"github.com/hashicorp/consul/api"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// ServerConfig Properties red from config yaml file
type ServerConfig struct {
	ConsulIP     string `yaml:"consulIP"`
	ConsulPort   string `yaml:"consulPort"`
	ConsulScheme string `yaml:"consulScheme"`
	NodeName     string `yaml:"nodeName"`
	ServiceName  string `yaml:"serviceName"`
	Location     string `yaml:"location"`
	Datacenter   string `yaml:"datacenter"`
	Type         string `yaml:"type"`
}

var srvConf ServerConfig

func findIPAdresses() (string, string) {

	// Private IP
	var nodeIP net.IP
	ifaces, err := net.Interfaces()
	for _, iface := range ifaces {
		addresses, _ := iface.Addrs()
		key := iface.Name
		for _, address := range addresses {
			if key == "br-bond0" {
				addIP, _, _ := net.ParseCIDR(address.String())
				if addIP.To4() != nil {
					nodeIP = addIP
				}
			}
		}
	}
	if err != nil {
		log.Fatalf("Error: %v\n", err)
	}

	privateIP := nodeIP.String()

	//2. Find Public IP

	resp, err := http.Get("http://ipinfo.io/ip")
	if err != nil {
		log.Fatalf("Error: %v\n", err)
	}
	defer resp.Body.Close()
	bodyBytes, _ := ioutil.ReadAll(resp.Body)
	responseBody := string(bodyBytes)

	publicIP := responseBody

	return privateIP, publicIP
}

func main() {
	log.Println("INFO: Bootstrapping Local ENEDI Master")

	//1. Find Private and Public IP Addresses of the (Virtual) Machine
	privateIP, publicIP := findIPAdresses()
	log.Println("INFO: Private IP of the machine: " + privateIP)
	log.Println("INFO: Public IP of the machine: " + publicIP)

	//2. Check if netdata is installed
	log.Println("INFO: Checking if netdata is installed")
	var out bytes.Buffer
	if _, errexists := os.Stat("/etc/netdata/netdata.conf"); errexists != nil {
		log.Println("INFO: Netdata is not installed. Installation will begin shortly...")

		//2a. Download the official script and run it
		//cmd := exec.Command("bash", "-c", "curl -Ss 'https://my-netdata.io/kickstart-static64.sh' > /tmp/kickstart.sh")
		cmd := exec.Command("bash", "-c", "curl -Ss 'https://my-netdata.io/kickstart.sh' > /tmp/kickstart.sh")
		cmd.Stdout = &out
		err := cmd.Run()
		if err != nil {
			log.Fatalf("Could not execute curl")
		}
		syscall.Chmod("/tmp/kickstart.sh", 0777)
		cmd = exec.Command("/tmp/kickstart.sh", "--dont-wait")
		if err = cmd.Run(); err != nil {
			log.Printf("Error %v\n", "Could not start process")
		}

		cmd = exec.Command("rm", "/tmp/kickstart.sh", "-rf")
		cmd.Stdout = &out
		err = cmd.Run()
		if err != nil {
			log.Fatalf("Could not remove script")
		}

		log.Println("INFO: Getting netdata configuration")

		time.Sleep(2 * time.Second)
		//2b. Get Netdata Configuration and update the [backend] prefix
		cmd = exec.Command("bash", "-c", "sudo wget -O /etc/netdata/netdata.conf 'http://localhost:19999/netdata.conf'")
		cmd.Stdout = &out
		err = cmd.Run()
		if err != nil {
			log.Fatalf("ERROR: Could not load netdata configuration ")
		}

		//2c. Edit Netdata Configuration
		input, err := ioutil.ReadFile("/etc/netdata/netdata.conf")
		if err != nil {
			log.Fatalf("ERROR: Reading netdata config: %v\n", err)
		}

		lines := strings.Split(string(input), "\n")

		// get hostname in order to use it in the metrics' prefix
		hostName, err := os.Hostname()
		if err != nil {
                        log.Fatalf("ERROR: Could not get hostname: %v\n", err)
                }

		for i, line := range lines {
			if strings.Contains(line, "# prefix = netdata") {
				lines[i] = "\tprefix = greece__heraklion__uoc__dc1__" + hostName
			}
		}
		output := strings.Join(lines, "\n")
		err = ioutil.WriteFile("/etc/netdata/netdata.conf", []byte(output), 0644)
		if err != nil {
			log.Fatalf("ERROR: Could not write netdata configuration: %v\n", err)
		}

		//2d. sudo service netdata restart
		cmd = exec.Command("bash", "-c", "service netdata restart")
		cmd.Stdout = &out
		err = cmd.Run()
		if err != nil {
			log.Fatalf("ERROR: Restart failed :%v\n", err)
		}
		log.Println("INFO: Netdata is successfully installed")

	} else {
		log.Println("INFO: Netdata is installed")
	}

	//3. Check for Local Consul Master
	log.Println("INFO: Setting up ENEDI monitoring infrastructure")
	cmd := exec.Command("bash", "-c", "docker-compose -f monitoring-infra/docker-compose.yaml up -d" )
	if err := cmd.Run(); err != nil {
		log.Printf("Error %v\n", "Could not start run ENEDI infrastructure compose")
	}


	//4. Insert Record to DB
	var config = &api.Config{
		Address: "localhost:8500",
		Scheme:  "http",
	}
	client, err := api.NewClient(config)
	if err != nil {
		log.Fatalf("Error: %v\n", err)
	}
	agent := client.Agent()
	var tags []string
	tags = append(tags, "uCatascopia")

	var agService = &api.AgentServiceRegistration{
		Address: privateIP,
		Port:    19999,
		Name:    "netdata",
		Tags:    tags,
	}

	// Sleep for a while - to ensure registration sleep duration: 30s
	log.Println("Waiting for Consul Server to initialize...")
	time.Sleep(10 * time.Second)
	// After sleep -- Register
	err = agent.ServiceRegister(agService)
	if err != nil {
		log.Fatalf("Error at Service Register: %v\n", err)
	}
	log.Println("Netdata service registered")



	//7. Connect to remote consul with some tags

}
