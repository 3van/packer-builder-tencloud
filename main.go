package main

import (
	"github.com/3van/packer-builder-tencloud/builder/tencloud"
	"github.com/hashicorp/packer/packer/plugin"
)

func main() {
	server, err := plugin.Server()
	if err != nil {
		panic(err)
	}
	server.RegisterBuilder(new(tencloud.Builder))
	server.Serve()
}
