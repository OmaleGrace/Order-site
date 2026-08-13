package main

import (
	"Order-site/database"
	"Order-site/menu"
	"fmt"
)

func main() {
	db, err := database.Connect()
	if err != nil {
		fmt.Println("Database connection failed:", err)
		return
	}
	defer db.Close()

	items, err := menu.GetAll(db)
	if err != nil {
		fmt.Println("Failed to get menu:", err)
		return
	}
	fmt.Println(items)
	fmt.Println("Database connected successfully!!")
}
