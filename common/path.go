package common

import (
	"flag"
	"os"
	"path"
	"runtime"
)

// 判断文件或路径是否存在
func Exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	return false
}

//配置项
type PathConfig struct {
	conf_path string
	Data_dir string
	User_dir string
	cwd string
}

var PathCfg PathConfig

func init() {
	PathCfg.Init()

	flag.StringVar(&PathCfg.conf_path, "config", PathCfg.conf_path, "setup config file path")
	flag.StringVar(&PathCfg.Data_dir, "datadir", PathCfg.Data_dir, "setup data files directory")
	flag.StringVar(&PathCfg.User_dir, "userdir", PathCfg.User_dir, "setup user files directory")
}

func (c *PathConfig) Init() {
	//cwd, _ := filepath.Abs(path.Dir(os.Args[0]))
	cwd := path.Dir(os.Args[0])
	//PathCfg.Data_dir = path.Join(cwd, "share", "gsnova")
	PathCfg.Data_dir = path.Join(cwd, "share")
	PathCfg.User_dir = path.Join(cwd, "user")
	//PathCfg.User_dir = UserHomeDir()
	//PathCfg.conf_path = path.Join(cwd, "data", "gsnova.conf")
	PathCfg.conf_path = path.Join(cwd, "share", "gsnova.conf")
}

func UserHomeDir() string {
	if runtime.GOOS == "windows" {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	}
	return os.Getenv("HOME")
}

