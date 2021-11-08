# LDAPServerMonitor

## Introduction
The LDAP server manages a large amount of data, but it cannot monitor the changes of these data.
`What is the use of monitoring data?`
Examples:
1. monitor the enabled/disabled of an account to handle subsequent account initialization/cleanup work.
2. Monitor the user's login operation, used to initialize some system data.
3. ...

## To start using LDAPServerMonitor
```go
func main() {
	monitor := ldapmonitor.NewMonitor(ldapmonitor.LDAPMonitorConfig{
		Url:                   "ldap://...",
		ManagerDN:             "",
		ManagerPassword:       "",
		BaseDN:                "",
		RefreshPeriodInSecond: 10,
		IgnoreLDAPFileds:      []string{"logonCount"},
	})

	monitor.AddListener("firstListener", func(data ldapmonitor.LDAPChangeEntry) {
		fmt.Printf("[%s] accountName: %s, filed: %s, changeType: %s, before: %s, after: %s\n",
		getTime(), data.AccountName, data.Filed, ldapmonitor.DataChangeTypeMap[int(data.ChangeType)],
		data.ChangeBefore, data.ChangeAfter)
	})

	monitor.Start()

	for {
		time.Sleep(time.Hour * 10)
	}
}

func getTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}
```

## Support
If you need support, raise an issue.
