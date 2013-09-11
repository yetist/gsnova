package proxy

import (
	"bufio"
	"github.com/yetist/gsnova/common"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"github.com/yetist/gsnova/util"
	"path"
)

const (
	DNS_CACHE_FILE     = "DNSCache.json"
	USER_HOSTS_FILE    = "user_hosts.conf"
	CLOUD_HOSTS_FILE   = "cloud_hosts.conf"
	HOSTS_DISABLE      = 0
	HOSTS_ENABLE_HTTPS = 1
	HOSTS_ENABLE_ALL   = 2
)

var repoUrls []string
var hostMapping = make(map[string]string)

var hostsEnable int
var trustedDNS = []string{}
var useHttpDNS = []*regexp.Regexp{}

var exceptHosts = []*regexp.Regexp{}

var hostRangeFetchLimitSize = uint32(256000)

//range fetch timeout secs
var rangeFetchTimeoutSecs = 90 * time.Second
var hostInjectRangePatterns = []*regexp.Regexp{}
var hostRangeConcurrentFether = uint32(5)

var hosts_data_dir = path.Join(common.PathCfg.Data_dir, "hosts")
var hosts_cache_dir = path.Join(common.PathCfg.User_dir, ".cache", "gsnova", "hosts")

func loadDiskHostFile(dir string) {
	files, err := ioutil.ReadDir(dir)
	if nil == err {
		for _, file := range files {
			switch file.Name() {
			case USER_HOSTS_FILE, CLOUD_HOSTS_FILE:
				continue
			}
			content, err := ioutil.ReadFile(path.Join(hosts_cache_dir, file.Name()))
			if nil == err {
				reader := bufio.NewReader(strings.NewReader(string(content)))
				for {
					line, _, err := reader.ReadLine()
					if nil != err {
						break
					}
					str := string(line)
					str = strings.TrimSpace(str)

					if strings.HasPrefix(str, "#") || len(str) == 0 {
						continue
					}
					ss := strings.Split(str, " ")
					if len(ss) == 1 {
						ss = strings.Split(str, "\t")
					}
					if len(ss) == 2 {
						k := strings.TrimSpace(ss[1])
						v := strings.TrimSpace(ss[0])
						if !isExceptHost(k) {
							hostMapping[k] = v
						}
					}
				}
			}
		}
	}
}

func loadHostFile() {
	hostMapping = make(map[string]string)
	loadDiskHostFile(hosts_data_dir)
	for index, urlstr := range repoUrls {
		resp, err := util.HttpGet(urlstr, "")
		if err != nil {
			if addr, exist := common.Cfg.GetProperty("LocalServer", "Listen"); exist {
				_, port, _ := net.SplitHostPort(addr)
				resp, err = util.HttpGet(urlstr, "http://"+net.JoinHostPort("127.0.0.1", port))
			}
		}
		if err != nil || resp.StatusCode != 200 {
			log.Printf("Failed to fetch host from %s\n", urlstr)
		} else {
			body, err := ioutil.ReadAll(resp.Body)
			if nil == err {
				hf := path.Join(hosts_cache_dir, "hosts_" + strconv.Itoa(index) + ".txt")
				ioutil.WriteFile(hf, body, 0755)
			}
		}
	}
	loadDiskHostFile(hosts_cache_dir)
}

func isExceptHost(host string) bool {
	return hostPatternMatched(exceptHosts, host)
}

func lookupAvailableAddress(hostport string) (string, bool) {
	host, port, err := net.SplitHostPort(hostport)
	if nil != err {
		return hostport, false
	}
	if addr, exist := trustedDNSQuery(host, port); exist {
		return net.JoinHostPort(addr, port), true
	}
	v, exist := getLocalHostMapping(host)
	if !exist {
		v, exist = hostMapping[host]
	}
	if exist && !isTCPAddressBlocked(v, port) {
		return net.JoinHostPort(v, port), true
	}
	return hostport, false
}

func lookupAvailableHostPort(req *http.Request, hostport string) (string, bool) {
	switch hostsEnable {
	case HOSTS_DISABLE:
		return "", false
	case HOSTS_ENABLE_HTTPS:
		if !strings.EqualFold(req.Method, "Connect") {
			return "", false
		}
	}
	return lookupAvailableAddress(hostport)
}

func hostNeedInjectRange(host string) bool {
	return hostPatternMatched(hostInjectRangePatterns, host)
}

func InitHosts() error {
	os.MkdirAll(hosts_cache_dir, 0755)
	loadLocalHostMappings(hosts_data_dir)
	if cloud_hosts, exist := common.Cfg.GetProperty("Hosts", "CloudHosts"); exist {
		go fetchCloudHosts(cloud_hosts)
	}

	if enable, exist := common.Cfg.GetIntProperty("Hosts", "Enable"); exist {
		hostsEnable = int(enable)
		if enable == 0 {
			return nil
		}
	}
	log.Println("Init AutoHost.")
	if dnsserver, exist := common.Cfg.GetProperty("Hosts", "TrustedDNS"); exist {
		trustedDNS = strings.Split(dnsserver, "|")
	}
	if timeout, exist := common.Cfg.GetIntProperty("Hosts", "BlockVerifyTimeout"); exist {
		blockVerifyTimeout = int(timeout)
	}
	if limit, exist := common.Cfg.GetIntProperty("Hosts", "RangeFetchLimitSize"); exist {
		hostRangeFetchLimitSize = uint32(limit)
	}
	if pattern, exist := common.Cfg.GetProperty("Hosts", "InjectRange"); exist {
		hostInjectRangePatterns = initHostMatchRegex(pattern)
	}
	if fetcher, exist := common.Cfg.GetIntProperty("Hosts", "RangeConcurrentFetcher"); exist {
		hostRangeConcurrentFether = uint32(fetcher)
	}
	if secs, exist := common.Cfg.GetIntProperty("Hosts", "RangeFetchTimeout"); exist {
		rangeFetchTimeoutSecs = time.Duration(secs) * time.Second
	}
	if crlfs, exist := common.Cfg.GetProperty("Hosts", "CRLF"); exist {
		crlfs = strings.Replace(crlfs, "\\r", "\r\r", -1)
		crlfs = strings.Replace(crlfs, "\\n", "\n\n", -1)
		CRLFs = []byte(crlfs)
	}

	if pattern, exist := common.Cfg.GetProperty("Hosts", "ExceptCloudHosts"); exist {
		exceptHosts = initHostMatchRegex(pattern)
	}
	repoUrls = make([]string, 0)
	index := 0
	for {
		v, exist := common.Cfg.GetProperty("Hosts", "CloudHostsRepo["+strconv.Itoa(index)+"]")
		if !exist || len(v) == 0 {
			break
		}
		repoUrls = append(repoUrls, v)
		index++
	}
	go loadHostFile()
	return nil
}
