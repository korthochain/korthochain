package ntp

import (
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/beevik/ntp"
	"github.com/gogf/gf/os/gproc"
	"github.com/gogf/gf/text/gstr"
)

var ntpPoool = map[string]struct{}{
	// cn
	"time1.cloud.tencent.com":  {},
	"time2.cloud.tencent.com":  {},
	"time3.cloud.tencent.com":  {},
	"time4.cloud.tencent.com":  {},
	"time5.cloud.tencent.com":  {},
	"ntp1.aliyun.com":          {},
	"ntp2.aliyun.com":          {},
	"ntp3.aliyun.com":          {},
	"ntp4.aliyun.com":          {},
	"ntp5.aliyun.com":          {},
	"ntp6.aliyun.com":          {},
	"ntp7.aliyun.com":          {},
	"ntp.sjtu.edu.cn":          {},
	"ntp.neu.edu.cn":           {},
	"ntp.bupt.edu.cn":          {},
	"ntp.shu.edu.cn":           {},
	"ntp.tuna.tsinghua.edu.cn": {},
	"cn.pool.ntp.org":          {},
	"0.cn.pool.ntp.org":        {},
	"1.cn.pool.ntp.org":        {},
	"2.cn.pool.ntp.org":        {},
	"3.cn.pool.ntp.org":        {},
	"hk.ntp.org.cn":            {},
	"tw.ntp.org.cn":            {},

	//global
	"pool.ntp.org":        {},
	"time.cloudflare.com": {},
	"time.google.com":     {},
	"time.apple.com":      {},
	"time.windows.com":    {},
	"jp.ntp.org.cn":       {},
}

func UpdateTimeFromNtp() error {
	var err error
	var resp *ntp.Response

	ticker := time.NewTicker(30 * time.Minute)
	go func() {
		for range ticker.C {
			for k, _ := range ntpPoool {
				resp, err = ntp.QueryWithOptions(k, ntp.QueryOptions{Timeout: 3 * time.Second, TTL: 30})
				if err != nil {
					fmt.Println(k+":", err)
					continue
				}
				break
			}

			if err != nil {
				continue
			}

			cst := time.FixedZone("CST", 8*3600)
			tstr := time.Now().Add(resp.ClockOffset).In(cst).Format("2006-01-02 15:04:05")

			if err := UpdateSystemDate(tstr); err != nil {
				log.Println("upadate date:", err)
				continue
			}

			fmt.Println("success set date:", tstr)
		}
	}()

	return nil
}

func UpdateSystemDate(dateTime string) error {
	system := runtime.GOOS
	switch system {
	case "windows":
		{
			_, err1 := gproc.ShellExec(`date  ` + gstr.Split(dateTime, " ")[0])
			_, err2 := gproc.ShellExec(`time  ` + gstr.Split(dateTime, " ")[1])
			if err1 != nil && err2 != nil {
				return err1
			}

		}
	case "linux":
		{
			_, err1 := gproc.ShellExec(`date -s  "` + dateTime + `"`)
			if err1 != nil {
				return err1
			}
		}
	case "darwin":
		{
			_, err1 := gproc.ShellExec(`date -s  "` + dateTime + `"`)
			if err1 != nil {
				return err1
			}
		}
	}

	return nil
}
