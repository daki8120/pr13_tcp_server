package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

type Registry struct {
	mu    sync.RWMutex
	users map[string]string // адрес сокета -> никнейм
}

var hub = Registry{users: make(map[string]string)}

func (r *Registry) Join(addr, nick string) {
	r.mu.Lock()
	r.users[addr] = nick
	r.mu.Unlock()
}

func (r *Registry) Leave(addr string) {
	r.mu.Lock()
	delete(r.users, addr)
	r.mu.Unlock()
}

func (r *Registry) List() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("--- Онлайн (%d) ---\n", len(r.users)))
	for addr, nick := range r.users {
		sb.WriteString(fmt.Sprintf(" * %s [%s]\n", nick, addr))
	}
	return sb.String()
}

func serveSession(conn net.Conn) {
	defer conn.Close()
	addr := conn.RemoteAddr().String()
	sc := bufio.NewScanner(conn)

	// 1. Запрос никнейма при входе (Обязательное задание)
	_, _ = fmt.Fprint(conn, "Введите ваш никнейм: ")
	if !sc.Scan() {
		return
	}
	nick := strings.TrimSpace(sc.Text())
	if nick == "" {
		nick = "Guest"
	}

	hub.Join(addr, nick)
	defer hub.Leave(addr)

	log.Printf("[+] Подключился %s (%s)", nick, addr)
	_, _ = fmt.Fprintf(conn, "Привет, %s! Команды: '/users' — кто онлайн, 'quit' — выход.\n", nick)

	// Скользящее окно времени для защиты от спама (не более 5 сообщ./сек)
	var msgTimes []time.Time

	for sc.Scan() {
		now := time.Now()
		msgTimes = append(msgTimes, now)
		if len(msgTimes) > 5 {
			msgTimes = msgTimes[1:]
			if now.Sub(msgTimes[0]) < time.Second {
				_, _ = fmt.Fprintln(conn, "⛔ Вы отключены за спам (лимит: 5 сообщений/сек)!")
				log.Printf("[!] %s (%s) кикнут за спам", nick, addr)
				return
			}
		}

		msg := strings.TrimSpace(sc.Text())
		log.Printf("[%s | %s]: %s", nick, addr, msg)

		switch msg {
		case "quit":
			_, _ = fmt.Fprintln(conn, "До свидания!")
			log.Printf("[-] %s (%s) вышел", nick, addr)
			return
		case "/users":
			_, _ = fmt.Fprint(conn, hub.List())
		default:
			_, _ = fmt.Fprintln(conn, "ECHO: "+strings.ToUpper(msg))
		}
	}
}

// Встроенный клиент для удобного тестирования без telnet
func runClient(addr string) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Fatalf("Не удалось подключиться: %v", err)
	}
	defer conn.Close()
	go func() { _, _ = io.Copy(os.Stdout, conn) }()
	_, _ = io.Copy(conn, os.Stdin)
}

func main() {
	port := flag.String("port", ":8080", "Порт сервера")
	isClient := flag.Bool("client", false, "Запустить в режиме тестового клиента")
	flag.Parse()

	if *isClient {
		runClient("localhost" + *port)
		return
	}

	ln, err := net.Listen("tcp", *port)
	if err != nil {
		log.Fatalf("Ошибка запуска TCP-сервера: %v", err)
	}
	defer ln.Close()
	log.Printf("🚀 TCP Эхо-сервер запущен на %s", *port)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Ошибка Accept: %v", err)
			continue
		}
		go serveSession(conn)
	}
}
