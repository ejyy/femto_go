package main

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
)

const TCP_PORT string = ":9000"

// TCP server managing client connections and exchange engine communication
type Server struct {
	engine    *Engine             // Exchange engine instance
	listener  net.Listener        // TCP listener
	clients   map[uint16]net.Conn // Active client connections by ID
	clientsMu sync.RWMutex        // Protects clients map
	nextConn  uint16              // Monotonic client ID generator
}

// Creates TCP server and binds to configured port
func NewServer(engine *Engine) *Server {
	listener, err := net.Listen("tcp", TCP_PORT)
	if err != nil {
		panic(err)
	}
	s := &Server{
		engine:   engine,
		listener: listener,
		clients:  make(map[uint16]net.Conn),
	}
	return s
}

// Accepts connections and processes engine I/O
func (s *Server) Start() {
	fmt.Println("TCP server started on port", TCP_PORT)

	// Accept client connections loop
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			continue
		}
		id := s.addClient(conn)
		go s.handleClient(conn, id)
	}
}

// Registers new connection and assigns unique ID
func (s *Server) addClient(conn net.Conn) uint16 {
	s.clientsMu.Lock()
	id := s.nextConn
	s.nextConn++
	s.clients[id] = conn
	s.clientsMu.Unlock()
	return id
}

// Removes connection from registry and closes socket
func (s *Server) delClient(conn net.Conn, id uint16) {
	s.clientsMu.Lock()
	delete(s.clients, id)
	s.clientsMu.Unlock()
	conn.Close()
}

// Manages individual client connection lifecycle
func (s *Server) handleClient(conn net.Conn, id uint16) {
	defer s.delClient(conn, id)
	s.handleMessage(conn, id)
}

// Processes incoming text commands from client
func (s *Server) handleMessage(conn net.Conn, id uint16) {
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		cmd := InputCommand{Trader: TraderID(id)}

		switch parts[0] {
		case "LIMIT": // LIMIT symbol side price size
			if len(parts) < 5 {
				continue
			}
			sym, _ := strconv.Atoi(parts[1])
			side, _ := strconv.Atoi(parts[2])
			price, _ := strconv.Atoi(parts[3])
			size, _ := strconv.Atoi(parts[4])

			cmd.Type = ORDER_EVENT
			cmd.Symbol = Symbol(sym)
			cmd.Side = Side(side)
			cmd.Price = Price(price)
			cmd.Size = Size(size)

		case "CANCEL": // CANCEL orderID
			if len(parts) < 2 {
				continue
			}
			oid, _ := strconv.Atoi(parts[1])
			cmd.Type = CANCEL_EVENT
			cmd.OrderID = OrderID(oid)

		case "QUIT": // Graceful disconnect
			s.delClient(conn, id)

		default:
			continue
		}

		s.engine.inputRing.Push(cmd)
	}
}

// Distributes events to all connected clients
func (s *Server) serverDistributionCallback(ev OutputEvent) {
	msg := fmt.Sprintf("%+v\n", ev)
	s.clientsMu.RLock()
	for _, c := range s.clients {
		c.Write([]byte(msg))
	}
	s.clientsMu.RUnlock()
}
