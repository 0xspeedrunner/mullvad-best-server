package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"runtime"
	"strings"
	"time"
	"sort"

	"github.com/go-ping/ping"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	var outputFlag = flag.String("o", "", "Output format. 'json' outputs server json")
	var topFlag = flag.Int("p", 10, "Top n server output")
	var countryFlag = flag.String("c", "ch", "Server country code, e.g. ch for Switzerland")
	var typeFlag = flag.String("t", "wireguard", "Server type, e.g. wireguard")
	var logLevel = flag.String("l", "info", "Log level. Allowed values: trace, debug, info, warn, error, fatal, panic")
	var stbootFlag = flag.Bool("st", true, "Only select diskless servers. Default: True")
	flag.Parse()

	level, err := zerolog.ParseLevel(*logLevel)
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to set log level")
	}
	zerolog.SetGlobalLevel(level)
	servers := getServers(*typeFlag)
	// TODO Remove BestServerIndex to get top n servers
	if *topFlag == 1 {
		bestIndex := selectBestServerIndex(servers, *countryFlag, *stbootFlag)
		if bestIndex == -1 {
			log.Fatal().Str("country", *countryFlag).Msg("No servers for country found.")
		}
		best := servers[bestIndex]
		log.Debug().Interface("server", best).Msg("Best latency server found.")
		hostname := strings.TrimSuffix(best.Hostname, "-wireguard")
		if *outputFlag == "json" {
			serverJson, err := json.Marshal(best)
			if err != nil {
				log.Fatal().Err(err).Msg("Couldn't marshal server information to Json")
			}
			fmt.Println(string(serverJson))
		} else {
			fmt.Println(hostname)
		}
	} else {
		bestServers := selectBestServers(servers, *countryFlag, *stbootFlag, *topFlag)
		for _, server := range bestServers {
			serverJson, err := json.Marshal(server)
			if err != nil {
				log.Fatal().Err(err).Msg("Couldn't marshal server information to Json")
			}
			fmt.Println(string(serverJson))
		}
	}
}

func selectBestServerIndex(servers []server, country string, stboot bool) int {
	bestIndex := -1
	var bestPing time.Duration
	for i, server := range servers {
// Make country flag optional - don't remove entirely
//		if server.Active && server.CountryCode == country && server.Stboot{
		if server.Active && server.Stboot{
			duration, err := serverLatency(server)
			if err == nil {
				if bestIndex == -1 || bestPing > duration {
					bestIndex = i
					bestPing = duration
				}
			} else {
				log.Err(err).Msg("Error determining the server latency via ping.")
			}
		}
	}
	return bestIndex
}

func selectBestServers(servers []server, country string, stboot bool, topFlag int) []serverDuration {

	var sortedServers []serverDuration
//	sortedServers = make([]serverDuration, topFlag)
//	var bestPing time.Duration
	for _, server := range servers {
// TODO: Make country flag optional - don't remove entirely
//		if server.Active && server.CountryCode == country && server.Stboot{
		if server.Active && server.Stboot{

			duration, err := serverLatency(server)
			if err == nil {
				sortedServers = append(sortedServers, serverDuration{server, duration})
			} else {
				log.Err(err).Msg("Error determining the server latency via ping.")
			}
		}
	}
	log.Debug().Msg("Pulled all servers")
	sort.SliceStable(sortedServers, func(i, j int) bool {
		return sortedServers[i].Duration < sortedServers[j].Duration
	})
	return sortedServers[:topFlag]
}

func getServers(serverType string) []server {
	resp, err := http.Get("https://api.mullvad.net/www/relays/" + serverType + "/")
	if err != nil {
		log.Fatal().Err(err).Msg("Couldn't retrieve servers")
	}
	responseBody, err := ioutil.ReadAll(resp.Body)
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Err(err)
		}
	}(resp.Body)
	if err != nil {
		log.Fatal().Err(err)
	}
	var servers []server
	err = json.Unmarshal(responseBody, &servers)
	if err != nil {
		log.Fatal().Err(err).Msg("couldn't unmarshall server json")
	}
	return servers
}

//goland:noinspection GoBoolExpressions
func serverLatency(s server) (time.Duration, error) {
	log.Debug().Str("Server", s.Hostname).Msg("Upcoming Server to Ping")
	pinger, err := ping.NewPinger(s.Ipv4AddrIn)
	if runtime.GOOS == "windows" {
		pinger.SetPrivileged(true)
	}
	pinger.Count = 1
	if err != nil {
		return 0, err
	}
	var duration time.Duration
	pinger.OnRecv = func(pkt *ping.Packet) {
		log.Debug().Str("Server", s.Hostname).IPAddr("IP", pkt.IPAddr.IP).Dur("RTT", pkt.Rtt).Msg("Added server latency.")
		duration = pkt.Rtt
	}
	pinger.Timeout = time.Second
	err = pinger.Run()
	if duration == 0 {
		return pinger.Timeout, err
	} else {
		return duration, err
	}
	

}

type server struct {
	Hostname         string `json:"hostname"`
	CountryCode      string `json:"country_code"`
	CountryName      string `json:"country_name"`
	CityCode         string `json:"city_code"`
	CityName         string `json:"city_name"`
	Active           bool   `json:"active"`
	Owned            bool   `json:"owned"`
	Provider         string `json:"provider"`
	Ipv4AddrIn       string `json:"ipv4_addr_in"`
	Ipv6AddrIn       string `json:"ipv6_addr_in"`
	NetworkPortSpeed int    `json:"network_port_speed"`
	Pubkey           string `json:"pubkey"`
	MultihopPort     int    `json:"multihop_port"`
	SocksName        string `json:"socks_name"`
	Stboot			 bool	`json:"stboot"`
}

type serverDuration struct {
	Server server
	Duration time.Duration
}
