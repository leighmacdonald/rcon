# RCON

Simple CLI & library for communicating with [Valve's Source RCON](https://developer.valvesoftware.com/wiki/Source_RCON_Protocol) 
compatible game servers (TF2/CS:GO/GMod/etc...).

## Command Line Usage

Basic flags for connecting. Host and password must be set. If the host does not specify a port, 27015 is used as default.

    Basic RCON CLI interface
    
    Usage:
      rcon [command] [flags]
    
    Flags:
      -h, --help              help for rcon
      -e  --env               Server environment(s) to use. Comma separated list of env names (eg: us-1,us-2)
      -H, --host string       Remote host, host:port format (default "localhost:27015")
      -p, --password string   RCON password
      -v, --version           version for rcon

If you do not specify a command a simple REPL shell will open instead as shown below:
    
    $ rcon -H tf2-server.com -p asdf       
    rcon> status
    hostname: Uncletopia | San Francisco
    version : 5970214/24 5970214 secure
    udp/ip  : 23.239.22.163:27015  (public ip: 23.239.22.163)
    steamid : [G:1:3414356] (85568392923453780)
    account : not logged in  (No account specified)
    map     : cp_sunshine at: 0 x, 0 y, 0 z
    tags    : Uncletopia,cp,nocrits,nodmgspread
    players : 23 humans, 0 bots (32 max)
    edicts  : 1028 used of 2048 max
    # userid name                uniqueid            connected ping loss state  adr
    #   1039 "BigDickMoe"        [U:1:356612105]     51:17      100    0 active 11.22.33.44:27005
    #   1058 "Yeooranium"        [U:1:279111806]     25:43      118    0 active 11.22.33.44:27005
    #   1062 "LaunderedPancake"  [U:1:87426245]      18:17       74    0 active 11.22.33.44:27005
    
    rcon> quit
    $
    
## Configuration

You can create a config file to save yourself from remembering all your server details each time.

The config file can be located at `~/rcon.yaml` or `./rcon.yaml`.

Below is an example demonstrating how to share common values `__defaults` among all servers.

```yaml
# When no server env is specified use these as the default
default_servers: [ "us-1", "us-2",  "us-3"]

# Set to "" to enable rcon console, otherwise this command will be executed and the program will exit
default_command: ""

# Example of sharing config values between hosts
__defaults: &defaults
    host: localhost:27015
    password: your_common_rcon_password

# Known server environments
servers:
    us-1:
        host: "us1.uncledane.com"
        password: custom_password
    us-2:
        <<: *defaults
        host: "us2.uncledane.com"
    us-3:
        <<: *defaults
        host: "us3.uncledane.com"
```

## Library Usage

```go
package main

import (
    "context"
    "fmt"
    "github.com/leighmacdonald/rcon/rcon"
    "log"
    "time"
)

func main() {
    // Connect
    conn, err := rcon.Dial(context.Background(), "localhost:27015", "P@SSW0RD", 10*time.Second)
    if err != nil {
        log.Fatalf("Failed to dial server")
    }
    // Exec your command
    resp, err := conn.Exec("status")
    if err != nil {
        log.Fatalf("Failed to exec command: %v", err)
    }
    // Do something with the response
    fmt.Printf("%s\n", resp)
}
```
