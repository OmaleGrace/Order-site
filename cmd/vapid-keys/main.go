// Command vapid-keys prints a new VAPID key pair for web push notifications.
//
//	go run ./cmd/vapid-keys
//
// Put the two values in the host's environment settings (Render > Environment). Create the pair
// once and keep it: changing the keys later makes every phone subscribe again.
package main

import (
	"fmt"
	"os"

	"Order-site/push"
)

func main() {
	pub, priv, err := push.GenerateKeys()
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not make keys:", err)
		os.Exit(1)
	}
	fmt.Println("Add these to your environment settings (the private key is a secret):")
	fmt.Println()
	fmt.Println("VAPID_PUBLIC_KEY=" + pub)
	fmt.Println("VAPID_PRIVATE_KEY=" + priv)
	fmt.Println("VAPID_SUBJECT=mailto:you@example.com")
}
