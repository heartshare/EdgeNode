package nodes

import (
	"github.com/TeaOSLab/EdgeCommon/pkg/nodeconfigs"
	"github.com/TeaOSLab/EdgeCommon/pkg/serverconfigs"
	"github.com/TeaOSLab/EdgeNode/internal/remotelogs"
	"github.com/iwind/TeaGo/lists"
	"net/url"
	"regexp"
	"sync"
)

var sharedListenerManager = NewListenerManager()

// 端口监听管理器
type ListenerManager struct {
	listenersMap map[string]*Listener // addr => *Listener
	locker       sync.Mutex
	lastConfig   *nodeconfigs.NodeConfig
}

// 获取新对象
func NewListenerManager() *ListenerManager {
	return &ListenerManager{
		listenersMap: map[string]*Listener{},
	}
}

// 启动监听
func (this *ListenerManager) Start(node *nodeconfigs.NodeConfig) error {
	this.locker.Lock()
	defer this.locker.Unlock()

	// 检查是否有变化
	/**if this.lastConfig != nil && this.lastConfig.Version == node.Version {
		return nil
	}**/
	this.lastConfig = node

	// 初始化
	err := node.Init()
	if err != nil {
		return err
	}

	// 所有的新地址
	groupAddrs := []string{}
	availableServerGroups := node.AvailableGroups()
	if !node.IsOn {
		availableServerGroups = []*serverconfigs.ServerGroup{}
	}

	if len(availableServerGroups) == 0 {
		remotelogs.Println("LISTENER_MANAGER", "no available servers to startup")
	}

	for _, group := range availableServerGroups {
		addr := group.FullAddr()
		groupAddrs = append(groupAddrs, addr)
	}

	// 停掉老的
	for listenerKey, listener := range this.listenersMap {
		addr := listener.FullAddr()
		if !lists.ContainsString(groupAddrs, addr) {
			remotelogs.Println("LISTENER_MANAGER", "close '"+addr+"'")
			_ = listener.Close()

			delete(this.listenersMap, listenerKey)
		}
	}

	// 启动新的或修改老的
	for _, group := range availableServerGroups {
		addr := group.FullAddr()
		listener, ok := this.listenersMap[addr]
		if ok {
			remotelogs.Println("LISTENER_MANAGER", "reload '"+this.prettyAddress(addr)+"'")
			listener.Reload(group)
		} else {
			remotelogs.Println("LISTENER_MANAGER", "listen '"+this.prettyAddress(addr)+"'")
			listener = NewListener()
			listener.Reload(group)
			err := listener.Listen()
			if err != nil {
				remotelogs.Error("LISTENER_MANAGER", err.Error())
				continue
			}
			this.listenersMap[addr] = listener
		}
	}

	return nil
}

// 获取总的活跃连接数
func (this *ListenerManager) TotalActiveConnections() int {
	this.locker.Lock()
	defer this.locker.Unlock()

	total := 0
	for _, listener := range this.listenersMap {
		total += listener.listener.CountActiveListeners()
	}
	return total
}

// 返回更加友好格式的地址
func (this *ListenerManager) prettyAddress(addr string) string {
	u, err := url.Parse(addr)
	if err != nil {
		return addr
	}
	if regexp.MustCompile(`^:\d+$`).MatchString(u.Host) {
		u.Host = "*" + u.Host
	}
	return u.String()
}
