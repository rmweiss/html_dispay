package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const mqttQoS byte = 1

type mqttController struct {
	client    mqtt.Client
	pageTopic string
}

func startMQTT(broker, pageTopic string, setPage func(string) error) (*mqttController, error) {
	if strings.TrimSpace(pageTopic) == "" {
		return nil, fmt.Errorf("MQTT page topic must not be empty")
	}

	controller := &mqttController{pageTopic: pageTopic}

	opts := mqtt.NewClientOptions().
		AddBroker(broker).
		SetClientID("html-display").
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(5 * time.Second)

	opts.OnConnect = func(client mqtt.Client) {
		log.Printf("MQTT connected to %s", broker)
		token := client.Subscribe(pageTopic, mqttQoS, func(_ mqtt.Client, message mqtt.Message) {
			url := strings.TrimSpace(string(message.Payload()))
			if url == "" {
				log.Printf("ignoring empty MQTT page URL on %s", pageTopic)
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

	return controller, nil
}

func (m *mqttController) publishPage(url string) error {
	token := m.client.Publish(m.pageTopic, mqttQoS, true, url)
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}
	log.Printf("published retained MQTT page URL to %s", m.pageTopic)
	return nil
}

func (m *mqttController) close() {
	if m == nil || m.client == nil {
		return
	}
	m.client.Disconnect(250)
}
