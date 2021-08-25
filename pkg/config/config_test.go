package config

import (
	"fmt"
	"testing"

	"github.com/spf13/viper"
)

func TestLoadConfig(t *testing.T) {
	cfg, _ := LoadConfig()
	t.Logf("%+v\n", cfg)
	t.Logf("%+v\n", cfg.LogConfig)

	// if cfg.ConsensusConfig == nil {
	// 	t.Log("raft is nil")
	// }

	if cfg.BFTConfig == nil {
		t.Fatal("bft is nil")
	} else {
		fmt.Printf("%+v", *(cfg.BFTConfig))
	}

	t.Log("blockchain_dexaddr", cfg.BlockchainConfig.DexFeeAddr)

}

func TestViper(t *testing.T) {
	viper.SetConfigName("korthoConf")
	viper.AddConfigPath("../configs/")
	if err := viper.ReadInConfig(); err != nil {
		t.Error(err)
	}

	var cfg CfgInfo
	if err := viper.Unmarshal(&cfg); err != nil {
		t.Error(err)
	}
	viper.Set("logconfig.level", "ERROR")
	//viper.MergeConfigMap(map[string]interface{}{"logconfig.level": "ERROR"})
	viper.WriteConfig()

	t.Logf("config info: %+v\n", cfg.LogConfig.Level)
}
