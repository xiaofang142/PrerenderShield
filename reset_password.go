package main

import (
	"encoding/json"
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
)

// User represents a user in the system
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func main() {
	// 读取当前用户配置
	file, err := os.ReadFile("data/users.json")
	if err != nil {
		fmt.Printf("Error reading users.json: %v\n", err)
		return
	}

	// 解析用户配置
	var users []User
	if err := json.Unmarshal(file, &users); err != nil {
		fmt.Printf("Error parsing users.json: %v\n", err)
		return
	}

	// 为admin用户生成新的密码哈希
	newPassword := "123456"
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		fmt.Printf("Error generating password hash: %v\n", err)
		return
	}

	// 更新admin用户的密码
	for i := range users {
		if users[i].Username == "admin" {
			users[i].Password = string(hash)
			break
		}
	}

	// 写入更新后的用户配置
	updatedData, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		fmt.Printf("Error marshalling users: %v\n", err)
		return
	}

	if err := os.WriteFile("data/users.json", updatedData, 0644); err != nil {
		fmt.Printf("Error writing users.json: %v\n", err)
		return
	}

	fmt.Printf("Password for admin user has been reset to '%s'\n", newPassword)
}