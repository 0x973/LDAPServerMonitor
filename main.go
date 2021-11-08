package main

import (
	"fmt"
	"main/ldapmonitor"
	"time"
)

func main() {
	monitor := ldapmonitor.NewMonitor(ldapmonitor.LDAPMonitorConfig{
		Url:                   "ldap://...",
		ManagerDN:             "",
		ManagerPassword:       "",
		BaseDN:                "",
		RefreshPeriodInSecond: 10,
		IgnoreLDAPFileds:      []string{"logonCount"},
	})

	monitor.AddListener("firstListener", listener)
	monitor.StartMonitor()

	for {
		time.Sleep(time.Hour * 10)
	}
}

func listener(data ldapmonitor.LDAPChangeEntry) {
	fmt.Printf("[%s] accountName: %s, filed: %s, changeType: %s, before: %s, after: %s\n",
		getTime(), data.AccountName, data.Filed, ldapmonitor.DataChangeTypeMap[int(data.ChangeType)],
		data.ChangeBefore, data.ChangeAfter)
}

func getTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}
