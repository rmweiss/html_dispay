package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const (
	mqttQoS            byte = 1
	mqttIPTopic             = "html-display/network/ip"
	mqttHeartbeatTopic      = "html-display/status/heartbeat"
	mqttHADiscovery         = "homeassistant/device/html-display/config"
	mqttHeartbeatInterval   = 5 * time.Second
)

type mqttController struct {
	client        mqtt.Client
	pageTopic     string
	heartbeatStop chan struct{}

	echoMu      sync.Mutex
	pendingEcho map[string]int
}

func startMQTT(broker, pageTopic string, setPage func(string) error) (*mqttController, error) {
	if strings.TrimSpace(pageTopic) == "" {
		return nil, fmt.Errorf("MQTT page topic must not be empty")
	}

	controller := &mqttController{
		pageTopic:     pageTopic,
		heartbeatStop: make(chan struct{}),
		pendingEcho:   make(map[string]int),
	}

	opts := mqtt.NewClientOptions().
		AddBroker(broker).
		SetClientID("html-display").
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(5 * time.Second)

	opts.OnConnect = func(client mqtt.Client) {
		log.Printf("MQTT connected to %s", broker)

		if ip, err := localIPv4ForBroker(broker); err != nil {
			log.Printf("could not determine local IP for MQTT metadata: %v", err)
		} else {
			token := client.Publish(mqttIPTopic, mqttQoS, true, ip)
			if token.Wait() && token.Error() != nil {
				log.Printf("could not publish MQTT IP address: %v", token.Error())
			} else {
				log.Printf("published retained MQTT IP address %s to %s", ip, mqttIPTopic)
			}
		}

		if err := publishHomeAssistantDiscovery(client, pageTopic); err != nil {
			log.Printf("could not publish Home Assistant discovery: %v", err)
		}

		token := client.Subscribe(pageTopic, mqttQoS, func(_ mqtt.Client, message mqtt.Message) {
			url := strings.TrimSpace(string(message.Payload()))
			if url == "" {
				log.Printf("ignoring empty MQTT page URL on %s", pageTopic)
				return
			}

			if controller.consumeEcho(url) {
				log.Printf("ignoring self-published MQTT page URL: %s", url)
				return
			}

			log.Printf("MQTT page request on %s: %s", pageTopic, url)
			if err := setPage(url); err != nil {
				log.Printf("MQTT navigation failed for %q: %v", url, err)
			}
		})
		if token.Wait() && token.Error() != nil {
			log.Printf("could not subscribe to MQTT page topic %s: %v", pageTopic, token.Error())
			return
		}
		log.Printf("subscribed to MQTT page topic %s", pageTopic)
	}

	opts.OnConnectionLost = func(_ mqtt.Client, err error) {
		log.Printf("MQTT connection lost: %v", err)
	}

	client := mqtt.NewClient(opts)
	controller.client = client

	token := client.Connect()
	if token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	go controller.runHeartbeat()

	return controller, nil
}

func (m *mqttController) runHeartbeat() {
	publish := func() {
		if !m.client.IsConnected() {
			return
		}
		token := m.client.Publish(mqttHeartbeatTopic, mqttQoS, false, "ON")
		if token.Wait() && token.Error() != nil {
			log.Printf("could not publish MQTT heartbeat: %v", token.Error())
		}
	}

	publish()
	ticker := time.NewTicker(mqttHeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			publish()
		case <-m.heartbeatStop:
			return
		}
	}
}

func publishHomeAssistantDiscovery(client mqtt.Client, pageTopic string) error {
	payload := map[string]any{
		"o": map[string]any{
			"name": "html-display",
		},
		"dev": map[string]any{
			"ids":  "html-display",
			"name": "HTML Display",
		},
		"cmps": map[string]any{
			"url": map[string]any{
				"p":             "text",
				"name":          "URL",
				"unique_id":     "html_display_url",
				"command_topic": pageTopic,
				"state_topic":   pageTopic,
				"retain":        true,
			},
			"ip": map[string]any{
				"p":               "sensor",
				"name":            "IP Address",
				"unique_id":       "html_display_ip",
				"state_topic":     mqttIPTopic,
				"entity_category": "diagnostic",
			},
			"online": map[string]any{
				"p":               "binary_sensor",
				"name":            "Online",
				"unique_id":       "html_display_online",
				"state_topic":     mqttHeartbeatTopic,
				"payload_on":      "ON",
				"expire_after":    15,
				"entity_category": "diagnostic",
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	token := client.Publish(mqttHADiscovery, mqttQoS, true, body)
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}
	log.Printf("published retained Home Assistant discovery to %s", mqttHADiscovery)
	return nil
}

func localIPv4ForBroker(broker string) (string, error) {
	u, err := url.Parse(broker)
	if err != nil {
		return "", err
	}

	host := u.Host
	if host == "" {
		return "", fmt.Errorf("MQTT broker URL has no host")
	}
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = net.JoinHostPort(u.Hostname(), "1883")
	}

	conn, err := net.Dial("udp4", host)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	local, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || local.IP == nil {
		return "", fmt.Errorf("could not determine local IPv4 address")
	}
	return local.IP.String(), nil
}

func (m *mqttController) markEcho(url string) {
	m.echoMu.Lock()
	defer m.echoMu.Unlock()
	m.pendingEcho[url]++
}

func (m *mqttController) consumeEcho(url string) bool {
	m.echoMu.Lock()
	defer m.echoMu.Unlock()

	count := m.pendingEcho[url]
	if count == 0 {
		return false
	}
	if count == 1 {
		delete(m.pendingEcho, url)
	} else {
		m.pendingEcho[url] = count - 1
	}
	return true
}

func (m *mqttController) discardEcho(url string) {
	m.echoMu.Lock()
	defer m.echoMu.Unlock()

	count := m.pendingEcho[url]
	if count <= 1 {
		delete(m.pendingEcho, url)
	} else {
		m.pendingEcho[url] = count - 1
	}
}

func (m *mqttController) publishPage(url string) error {
	m.markEcho(url)
	token := m.client.Publish(m.pageTopic, mqttQoS, true, url)
	if token.Wait() && token.Error() != nil {
		m.discardEcho(url)
		return token.Error()
	}
	log.Printf("published retained MQTT page URL to %s", m.pageTopic)
	return nil
}

func (m *mqttController) close() {
	if m == nil || m.client == nil {
		return
	}
	close(m.heartbeatStop)
	m.client.Disconnect(250)
}
