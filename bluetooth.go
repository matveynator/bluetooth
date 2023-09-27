package main

import (
  "context"
  "log"
  "os"
  "os/signal"
  "time"

  "github.com/go-ble/ble"
  "github.com/go-ble/ble/examples/lib/dev"
  "github.com/pkg/errors"
)

const (
  deviceName        = "RadiaCode-101#RC-101-005020"
  serviceUUID       = "1185978924B0B096D8437070E51532E6"
  connectionRetry   = 5 * time.Second
  connectionTimeout = 5 * time.Second
)

func main() {
  d, err := dev.NewDevice("default")
  if err != nil {
    log.Fatalf("Can't create device: %v", err)
  }
  ble.SetDefaultDevice(d)

  sigCh := make(chan os.Signal, 1)
  signal.Notify(sigCh, os.Interrupt)

  go func() {
    log.Println("Searching for device...")
    for {
      select {
      case <-sigCh:
        return
      default:
        ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
        defer cancel() // Освобождаем ресурсы контекста

        err := ble.Scan(ctx, true, func(a ble.Advertisement) {
          if a.LocalName() == deviceName {
            err := connect(a.Addr())
            if err != nil {
              log.Printf("Error connecting to device: %v", err)
            }
          }
        }, nil)

        if err != nil && err != context.DeadlineExceeded {
          log.Printf("Error scanning for devices: %v", err)
        }

        if err == context.DeadlineExceeded {
          log.Println("Scan timeout, retrying...")
        }

        time.Sleep(connectionRetry)
      }
    }
  }()

  select {
  case <-sigCh:
    log.Println("Interrupted")
  }
}

func connect(addr ble.Addr) error {
  log.Printf("Found device: %s, connecting...", deviceName)
  ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
  defer cancel()

  client, err := ble.Connect(ctx, func(a ble.Advertisement) bool {
    return a.Addr().String() == addr.String()
  })
  if err != nil {
    return errors.Wrap(err, "Can't connect to device")
  }
  log.Println("Connected to device:", addr)

  uuid, _ := ble.Parse(serviceUUID)
  service, err := client.DiscoverServices([]ble.UUID{uuid})
  if err != nil {
    return errors.Wrap(err, "Can't discover service")
  }
  if len(service) == 0 {
    return errors.New("Service not found")
  }

  characteristics, err := client.DiscoverCharacteristics(nil, service[0])
  if err != nil {
    return errors.Wrap(err, "Can't discover characteristics")
  }
  for _, c := range characteristics {
    if err := client.Subscribe(c, false, func(b []byte) {
      log.Printf("Received notification: %x", b)
    }); err != nil {
      return errors.Wrap(err, "Can't subscribe to characteristic")
    }
  }

  return nil
}

