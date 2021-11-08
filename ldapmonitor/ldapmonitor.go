package ldapmonitor

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/Workiva/go-datastructures/set"
	"github.com/go-ldap/ldap"
)

const _accountNameFiled = "sAMAccountName"

type Monitor struct {
	isClose           bool
	conn              *ldap.Conn
	ldapMonitorConfig LDAPMonitorConfig
	listeners         sync.Map
	dataBefore        map[string]map[string]string
	dataAfter         map[string]map[string]string
	dataChan          chan LDAPChangeEntry
	ignoreFileds      set.Set
}

type DataChangeType int32

const (
	Create DataChangeType = iota
	Delete
	Modify
)

var DataChangeTypeMap = map[int]string{
	int(Create): "Create",
	int(Delete): "Delete",
	int(Modify): "Modify",
}

type LDAPChangeEntry struct {
	AccountName  string         // Mapping for LDAP: sAMAccountName(unique)
	Filed        string         // LDAP Server FiledName
	ChangeBefore string         // The before change data
	ChangeAfter  string         // The after change data
	ChangeType   DataChangeType // Data change type
}

func newLDAPChangeEntry(accountName, filed, before, after string, changeType DataChangeType) LDAPChangeEntry {
	return LDAPChangeEntry{
		AccountName:  accountName,
		Filed:        filed,
		ChangeBefore: before,
		ChangeAfter:  after,
		ChangeType:   changeType,
	}
}

// LDAPMonitorConfig
type LDAPMonitorConfig struct {
	Url                   string   // LDAP Server URL
	ManagerDN             string   // LDAP Server account
	ManagerPassword       string   // LDAP Server account password
	BaseDN                string   // Base DN
	RefreshPeriodInSecond int      // Background Refresh cycle, default is 20 seconds
	IgnoreLDAPFileds      []string // Ignore LDAP fileds
	PrintLog              bool     // Log output switch
}

// Initialize the LDAP monitor
func NewMonitor(config LDAPMonitorConfig) Monitor {
	monitor := &Monitor{
		ldapMonitorConfig: config,
		conn:              connectLDAPServer(config),
		isClose:           false,
		dataChan:          make(chan LDAPChangeEntry, 1024),
	}

	if config.IgnoreLDAPFileds != nil {
		for _, v := range config.IgnoreLDAPFileds {
			monitor.ignoreFileds.Add(v)
		}
	}

	if config.RefreshPeriodInSecond <= 0 {
		config.RefreshPeriodInSecond = 20
	}

	return *monitor
}

// Add the listener function for the LDAP monitor
func (m *Monitor) AddListener(listenerName string, f func(LDAPChangeEntry)) error {
	if len(listenerName) == 0 {
		return errors.New("'listenerName' do not be empty")
	}

	m.listeners.Store(listenerName, f)
	return nil
}

// Remove the listener function for the LDAP monitor
func (m *Monitor) RemoveListener(listenerName string) error {
	if len(listenerName) == 0 {
		return errors.New("'listenerName' do not be empty")
	}

	m.listeners.Delete(listenerName)
	return nil
}

// Start work for the LDAP monitor
func (m *Monitor) StartMonitor() {
	go m.backgroundMonitor()
	go m.dispatch()
	m.printLog("LDAP Monitor Started.")
}

// Close the LDAP monitor
func (m *Monitor) Close() {
	if m.isClose || m.conn.IsClosing() {
		return
	}

	m.conn.Close()
	close(m.dataChan)
	m.isClose = true
}

func connectLDAPServer(config LDAPMonitorConfig) *ldap.Conn {
	conn, err := ldap.DialURL(config.Url)
	if err != nil {
		panic(err)
	}

	err = conn.Bind(config.ManagerDN, config.ManagerPassword)
	if err != nil {
		panic(err)
	}

	return conn
}

func (m *Monitor) diffAccountName() {
	before := m.dataBefore
	after := m.dataAfter
	accountNamesBefor := getKeysOfMaps(before)
	accountNamesAfter := getKeysOfMaps(after)
	allUserNames := union(accountNamesBefor, accountNamesAfter)
	for _, accountName := range allUserNames {
		userBefore, existBefore := before[accountName]
		userAfter, existAfter := after[accountName]
		if existBefore && !existAfter {
			m.deleteUserAction(accountName, userBefore)
		} else if !existBefore && existAfter {
			m.createUserAction(accountName, userAfter)
		} else if existBefore && existAfter {
			m.diffAllFileds(accountName, userBefore, userAfter)
		}
	}
}

func (m *Monitor) diffAllFileds(accountName string, userBefore, userAfter map[string]string) {
	filedsBefor := getKeysOfMap(userBefore)
	filedsAfter := getKeysOfMap(userAfter)
	allFileds := union(filedsBefor, filedsAfter)

	for _, filed := range allFileds {
		if m.ignoreFileds.Exists(filed) {
			continue
		}

		stringBefore, existBefore := userBefore[filed]
		stringAfter, existAfter := userAfter[filed]
		if existBefore && !existAfter {
			// delete filed
			m.changeAction(accountName, filed, stringBefore, stringAfter, Delete)
		} else if !existBefore && existAfter {
			// create filed
			m.changeAction(accountName, filed, stringBefore, stringAfter, Create)
		} else if existBefore && existAfter && stringBefore != stringAfter {
			// modify filed
			m.changeAction(accountName, filed, stringBefore, stringAfter, Modify)
		}
	}
}

func (m *Monitor) deleteUserAction(accountName string, data map[string]string) {
	for filed, value := range data {
		m.changeAction(accountName, filed, value, "", Delete)
	}
}

func (m *Monitor) createUserAction(accountName string, data map[string]string) {
	for filed, value := range data {
		m.changeAction(accountName, filed, "", value, Create)
	}
}

func (m *Monitor) changeAction(accountName, filed, valueBefore, valueAfter string, changeType DataChangeType) {
	m.dataChan <- newLDAPChangeEntry(accountName, filed, valueBefore, valueAfter, changeType)
}

func (m *Monitor) backgroundMonitor() {
	for !m.isClose {
		m.dataAfter = m.getAllData()
		if len(m.dataBefore) == 0 {
			m.dataBefore = m.dataAfter
			continue
		}

		m.diffAccountName()
		m.dataBefore = m.dataAfter
		time.Sleep(time.Second * time.Duration(m.ldapMonitorConfig.RefreshPeriodInSecond))
	}
}

func (m *Monitor) dispatch() {
	for data := range m.dataChan {
		m.listeners.Range(func(key, function interface{}) bool {
			go function.(func(LDAPChangeEntry))(data)
			return true
		})
	}
}

func (m *Monitor) getAllData() map[string]map[string]string {
	result := map[string]map[string]string{}

	const pageSize uint32 = 512
	searchBase := m.ldapMonitorConfig.BaseDN
	filter := "(objectClass=*)"
	attributes := []string{}

	m.printLog("Starting get data from LDAP Server")
	defer m.printLog("Ending get data from LDAP Server")

	m.searchAll(pageSize, searchBase, filter, attributes, func(entries []*ldap.Entry) {
		for _, entry := range entries {
			attributes := make(map[string]string, len(entry.Attributes))
			for _, attribute := range entry.Attributes {
				attributes[attribute.Name] = fmt.Sprintf("%s", attribute.Values)
			}
			name := entry.GetAttributeValue(_accountNameFiled)
			result[name] = attributes
		}
	})

	return result
}

func (m *Monitor) searchAll(pageSize uint32, searchBase string, filter string, attributes []string, callback func([]*ldap.Entry)) {
	pagingControl := ldap.NewControlPaging(pageSize)
	controls := []ldap.Control{pagingControl}
	for {
		request := ldap.NewSearchRequest(searchBase, ldap.ScopeWholeSubtree, ldap.DerefAlways, 0, 0, false, filter, attributes, controls)
		response, err := m.conn.Search(request)
		if err != nil {
			if strings.Contains(err.Error(), "LDAP Result Code 200") {
				m.printLog("Reconnecting...")
				m.conn = connectLDAPServer(m.ldapMonitorConfig)
				time.Sleep(time.Millisecond * 500)
				continue
			}

			m.printLog(fmt.Sprintf("Failed to execute search request: %s\n", err.Error()))
			continue
		}

		callback(response.Entries)

		updatedControl := ldap.FindControl(response.Controls, ldap.ControlTypePaging)
		if ctrl, ok := updatedControl.(*ldap.ControlPaging); ok && ctrl != nil && len(ctrl.Cookie) != 0 {
			pagingControl.SetCookie(ctrl.Cookie)
			continue
		}
		break
	}
}

func (m *Monitor) printLog(message string) {
	if m.ldapMonitorConfig.PrintLog {
		log.Println(message)
	}
}
