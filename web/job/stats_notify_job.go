package job

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"time"

	"x-ui/logger"
	"x-ui/util/common"
	"x-ui/web/service"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var SSHLoginUser int

type LoginStatus byte

const (
	LoginSuccess LoginStatus = 1
	LoginFail    LoginStatus = 0
)

type StatsNotifyJob struct {
	enable         bool
	xrayService    service.XrayService
	inboundService service.InboundService
	settingService service.SettingService
}

func NewStatsNotifyJob() *StatsNotifyJob {
	return new(StatsNotifyJob)
}

func (j *StatsNotifyJob) SendMsgToTgbot(msg string) {
	//Telegram bot basic info
	tgBottoken, err := j.settingService.GetTgBotToken()
	if err != nil {
		logger.Warning("sendMsgToTgbot failed,GetTgBotToken fail:", err)
		return
	}
	tgBotid, err := j.settingService.GetTgBotChatId()
	if err != nil {
		logger.Warning("sendMsgToTgbot failed,GetTgBotChatId fail:", err)
		return
	}
	if tgBottoken == "" || tgBotid == 0 {
		return
	}

	bot, err := tgbotapi.NewBotAPI(tgBottoken)
	if err != nil {
		fmt.Println("get tgbot error:", err)
		return
	}
	bot.Debug = true
	fmt.Printf("Authorized on account %s", bot.Self.UserName)
	info := tgbotapi.NewMessage(int64(tgBotid), msg)
	//msg.ReplyToMessageID = int(tgBotid)
	bot.Send(info)
}

//Here run is a interface method of Job interface
func (j *StatsNotifyJob) Run() {
	if !j.xrayService.IsXrayRunning() {
		return
	}
	var info string
	//get hostname
	name, err := os.Hostname()
	if err != nil {
		fmt.Println("get hostname error:", err)
		return
	}
	info = fmt.Sprintf("主机名称:%s\r\n", name)
	//get ip address
	var ip string
	netInterfaces, err := net.Interfaces()
	if err != nil {
		fmt.Println("net.Interfaces failed, err:", err.Error())
		return
	}

	for i := 0; i < len(netInterfaces); i++ {
		if (netInterfaces[i].Flags & net.FlagUp) != 0 {
			addrs, _ := netInterfaces[i].Addrs()

			for _, address := range addrs {
				if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					if ipnet.IP.To4() != nil {
						ip = ipnet.IP.String()
						break
					} else {
						ip = ipnet.IP.String()
						break
					}
				}
			}
		}
	}
	info += fmt.Sprintf("IP地址:%s\r\n \r\n", ip)

	//get traffic
	inbouds, err := j.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("StatsNotifyJob run failed:", err)
		return
	}
	//NOTE:If there no any sessions here,need to notify here
	//TODO:分节点推送,自动转化格式
	for _, inbound := range inbouds {
		info += fmt.Sprintf("节点名称:%s\r\n端口:%d\r\n上行流量↑:%s\r\n下行流量↓:%s\r\n总流量:%s\r\n", inbound.Remark, inbound.Port, common.FormatTraffic(inbound.Up), common.FormatTraffic(inbound.Down), common.FormatTraffic((inbound.Up + inbound.Down)))
		if inbound.ExpiryTime == 0 {
			info += fmt.Sprintf("到期时间:无限期\r\n \r\n")
		} else {
			info += fmt.Sprintf("到期时间:%s\r\n \r\n", time.Unix((inbound.ExpiryTime/1000), 0).Format("2006-01-02 15:04:05"))
		}
	}
	j.SendMsgToTgbot(info)
}

func (j *StatsNotifyJob) UserLoginNotify(username string, ip string, time string, status LoginStatus) {
	if username == "" || ip == "" || time == "" {
		logger.Warning("UserLoginNotify failed,invalid info")
		return
	}
	var msg string
	//get hostname
	name, err := os.Hostname()
	if err != nil {
		fmt.Println("get hostname error:", err)
		return
	}
	if status == LoginSuccess {
		msg = fmt.Sprintf("面板登录成功提醒\r\n主机名称:%s\r\n", name)
	} else if status == LoginFail {
		msg = fmt.Sprintf("面板登录失败提醒\r\n主机名称:%s\r\n", name)
	}
	msg += fmt.Sprintf("时间:%s\r\n", time)
	msg += fmt.Sprintf("用户:%s\r\n", username)
	msg += fmt.Sprintf("IP:%s\r\n", ip)
	j.SendMsgToTgbot(msg)
}

func (j *StatsNotifyJob) SSHStatusLoginNotify() {
	getSSHUserNumber, error := exec.Command("bash", "-c", "who | awk  '{print $1}'|wc -l").Output()
	if error != nil {
		fmt.Println("getSSHUserNumber error:", error)
		return
	}
	var numberInt int
	numberInt, error = strconv.Atoi(common.ByteToString(getSSHUserNumber))
	if error != nil {
		return
	}
	if numberInt > SSHLoginUser {
		var SSHLoginInfo string
		SSHLoginUser = numberInt
		//hostname
		name, err := os.Hostname()
		if err != nil {
			fmt.Println("get hostname error:", err)
			return
		}
		//Time compare,needed if x-ui got restart while there already exist ssh users
		SSHLoginTime, error := exec.Command("bash", "-c", "who | awk  '{print $3,$4}'|tail -n 1 ").Output()
		if error != nil {
			fmt.Println("getLoginTime error:", error.Error())
			return
		}
		SSHLoginUserName, error := exec.Command("bash", "-c", "who | awk  '{print $1}'|tail -n 1").Output()
		if error != nil {
			fmt.Println("getSSHLoginUserName error:", error.Error())
			return
		}
		SSHLoginIpAddr, error := exec.Command("bash", "-c", "who | awk  '{print $5}'|tail -n 1 | cut -d \"(\" -f2 | cut -d \")\" -f1 ").Output()
		if error != nil {
			fmt.Println("getSSHLoginIpAddr error:", error)
			return
		}

		SSHLoginInfo = fmt.Sprintf("SSH用户登录提醒:\r\n")
		SSHLoginInfo += fmt.Sprintf("主机名称:%s\r\n", name)
		SSHLoginInfo += fmt.Sprintf("SSH登录用户:%s", SSHLoginUserName)
		SSHLoginInfo += fmt.Sprintf("SSH登录时间:%s", SSHLoginTime)
		SSHLoginInfo += fmt.Sprintf("SSH登录IP:%s", SSHLoginIpAddr)
		SSHLoginInfo += fmt.Sprintf("当前SSH登录用户数:%s", getSSHUserNumber)
		j.SendMsgToTgbot(SSHLoginInfo)
	} else {
		SSHLoginUser = numberInt
	}
}
