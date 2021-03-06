package stats

import (
	"compress/gzip"
	"encoding/gob"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aarondl/ultimateq/irc"
)

func init() {
	rand.Seed(time.Now().Unix())
}

var fileOpener FileOpener = osFileOpener{}

type Stats struct {
	Channels map[uint]*Channel
	Networks map[uint]*Network
	Users    map[uint]*User

	networkByName map[string]*Network

	NetworkIDCount uint
	MessageIDCount uint
	ChannelIDCount uint
	UserIDCount    uint

	mut sync.RWMutex
}

// NewStats initializes a Stats struct.
func NewStats() *Stats {
	s, err := loadDatabase()

	if err != nil {
		fmt.Printf("Error'd: %v\n", err)
		return nil
	}

	if s != nil {
		return s
	}

	// load from stats.db
	return &Stats{
		Channels: make(map[uint]*Channel),
		Networks: make(map[uint]*Network),
		Users:    make(map[uint]*User),

		networkByName: make(map[string]*Network),

		NetworkIDCount: 1,
		MessageIDCount: 1,
		ChannelIDCount: 1,
		UserIDCount:    1,
	}
}

// GetNetwork retrieves a network by its name return nil if not found
func (s *Stats) GetNetwork(network string) *Network {
	return s.networkByName[network]
}

// GetChannel retrieves a channel from the specified network by name
func (s *Stats) GetChannel(network, channel string) *Channel {
	if n := s.GetNetwork(network); n != nil {
		return n.channels[channel]
	}

	return nil
}

// GetUser retrieves a user from the specified network by name
func (s *Stats) GetUser(network, nick string) *User {
	if n := s.GetNetwork(network); n != nil {
		return n.users[nick]
	}

	return nil
}

// AddMessage adds a message to the stats.
func (s *Stats) AddMessage(kind MsgKind, network string, channel string, hostmask string, date time.Time, message string) {

	var c *Channel
	var cu *User

	n := s.getNetwork(network)
	u := s.getUser(n, hostmask)

	// channel can be blank (for example a QUIT message has no channel)
	if channel != "" {
		c = s.getChannel(n, channel)
		cu = s.getChannelUser(u, channel)
	}

	s.addMessage(kind, n, c, u, cu, date, message)
}

func (s *Stats) addMessage(k MsgKind, n *Network, c *Channel, u *User, cu *User, d time.Time, m string) *Message {
	id := s.MessageIDCount
	s.MessageIDCount++

	message := &Message{
		ID:        id,
		Date:      d,
		UserID:    u.ID,
		ChannelID: 0,
		Message:   m,
		Kind:      k,
	}

	if c != nil {
		message.ChannelID = c.ID
		c.addMessage(n, message, u)

		switch k {
		case Kick:
			c.addKick(s, message)
		case Action:
			c.addAction(s, message)
		}

		if cu != nil {
			cu.addMessage(n, c, message)
		}
	}

	n.addMessage(message)
	u.addMessage(n, c, message)

	return message
}

func (s *Stats) addChannel(n *Network, name string) *Channel {
	id := s.ChannelIDCount
	s.ChannelIDCount++

	c := newChannel(id, n, name)

	s.Channels[c.ID] = c

	n.addChannel(c)

	return c
}

func (s *Stats) addUser(n *Network, nick string) *User {
	id := s.UserIDCount
	s.UserIDCount++

	u := NewUser(id, n.ID, nick)

	s.Users[id] = u

	n.addUser(u)

	return u
}

// getChannelUser
func (s *Stats) getChannelUser(user *User, channel string) *User {
	channel = strings.ToLower(channel)
	if cu, ok := user.ChannelUsers[channel]; ok {
		return cu
	} else {
		return user.addChannelUser(channel)
	}
}

func (s *Stats) getUser(n *Network, nameOrHost string) *User {
	nick := irc.Nick(nameOrHost)

	if u, ok := n.users[strings.ToLower(nick)]; ok {
		return u
	} else {
		return s.addUser(n, nick)
	}
}

func (s *Stats) getChannel(n *Network, name string) *Channel {
	if c, ok := n.channels[strings.ToLower(name)]; ok {
		return c
	} else {
		return s.addChannel(n, name)
	}
}

func (s *Stats) getNetwork(name string) *Network {
	if n, ok := s.networkByName[strings.ToLower(name)]; ok {
		return n
	} else {
		return s.addNetwork(name)
	}
}

func (s *Stats) addNetwork(name string) *Network {
	id := s.NetworkIDCount
	s.NetworkIDCount++

	n := &Network{
		Name:        name,
		ID:          id,
		stats:       s,
		ChannelIDs:  make([]uint, 0),
		UserIDs:     make([]uint, 0),
		MessageIDs:  make([]uint, 0),
		URLCounter:  NewURLCounter(),
		WordCounter: NewWordCounter(),

		channels: make(map[string]*Channel),
		users:    make(map[string]*User),
	}

	s.Networks[id] = n
	s.networkByName[strings.ToLower(name)] = n

	return n
}

// Save writes the statistics to data.db.
func (s *Stats) Save() bool {
	f, _ := fileOpener.Create("data.db")
	defer f.Close()

	gz := gzip.NewWriter(f)
	defer gz.Close()

	enc := gob.NewEncoder(gz)
	err := enc.Encode(s)

	if err != nil {
		log.Fatal("encode error:", err)
		return false
	}

	return true
}

// buildIndexes builds the internal maps that relate data
func (s *Stats) buildIndexes() {
	s.networkByName = make(map[string]*Network)

	for _, n := range s.Networks {
		s.networkByName[n.Name] = n
		n.buildIndexes(s)
	}
}

// loadDatabase reads data.db and populates a Stats struct.
func loadDatabase() (*Stats, error) {
	file, err := fileOpener.Open("./data.db")
	defer file.Close()

	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		} else {
			fmt.Println("Some other error: %v", err)
			return nil, err
		}
	}

	r, _ := gzip.NewReader(file)
	defer r.Close()
	decoder := gob.NewDecoder(r)
	var stats Stats

	if err = decoder.Decode(&stats); err != nil {
		return nil, err
	}

	stats.buildIndexes()

	return &stats, nil
}

// Lock proxies the RWMutex's Lock function.
func (s *Stats) Lock() {
	s.mut.Lock()
}

// Unlock proxies the RWMutex's Unlock function.
func (s *Stats) Unlock() {
	s.mut.Unlock()
}

// RLock proxies the RWMutex's RLock function.
func (s *Stats) RLock() {
	s.mut.RLock()
}

// RUnlock proxies the RWMutex's Unlock function.
func (s *Stats) RUnlock() {
	s.mut.RUnlock()
}
