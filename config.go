package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
)

const version = "0.0.9"
const programname = "idbvmware"
const exampleFilename = programname + ".json.example"

var configFile = flag.String("config", "/etc/bytemine/"+programname+".json", "config file")
var writeExample = flag.Bool("example", false, "write an example config to "+exampleFilename+" in the current dir.")
var showVersion = flag.Bool("version", false, "display version and exit")
var dryrun = flag.Bool("dryrun", false, "do noting in the idb")

func init() {
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	if *writeExample {
		c := example()
		err := writeConfig(exampleFilename, c)
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}
}

func errEmptyValue(e string) error {
	return errors.New(fmt.Sprintf("%v can't be empty!", e))
}

type config struct {
	// Create machines if they don't exists in the IDB
	Create bool

	// IDB API URL, eg. https://idb.example.com
	IdbUrl string

	// IDB API token
	IdbToken string

	// VMware API URL, eg. https://root:password@localhost:443/sdk
	VmwareUrl string

	// Try to do a reverse lookup for machines with invalid or unknown fqdn
	Lookup bool

	// Suffix for machines with invalid or unknown fqdn
	UnknownSuffix string

	// Strip invalid characters from fqdn
	FqdnStrip bool

	// Ignore invalid SSL chains
	InsecureSkipVerify bool

	// Debug mode
	Debug bool
}

func (c *config) check() error {
	if c.IdbUrl == "" {
		return errEmptyValue("ApiUrl")
	}

	if c.IdbToken == "" {
		return errEmptyValue("ApiToken")
	}

	if c.VmwareUrl == "" {
		return errEmptyValue("VmwareUrl")
	}

	if c.UnknownSuffix == "" {
		return errEmptyValue("UnknownSuffix")
	}

	return nil
}

func example() *config {
	c := &config{}
	c.Create = true
	c.IdbUrl = "https://idb.example.com"
	c.IdbToken = "mySecretIdbToken"
	c.VmwareUrl = "https://root:password@localhost:443/sdk"
	c.Lookup = true
	c.UnknownSuffix = ".vmware.example.com"
	c.InsecureSkipVerify = false
	c.Debug = true
	c.FqdnStrip = true
	return c
}

func loadConfig(filename string) (*config, error) {
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	c := &config{}
	err = json.Unmarshal(buf, c)
	if err != nil {
		return nil, err
	}

	err = c.check()
	if err != nil {
		return nil, err
	}

	return c, nil
}

func writeConfig(file string, c *config) error {
	buf, err := json.MarshalIndent(c, "", "\t")
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(file, buf, 0600)
	return err
}
