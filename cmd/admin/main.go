package main

import (
	"campusclaw/internal/config"
	"campusclaw/internal/provision"
	repository "campusclaw/internal/repository/mysql"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

func run() error {
	mode := flag.String("mode", "", "schema, fresh, class, or user")
	name := flag.String("name", "", "class name or username")
	role := flag.String("role", "", "teacher or student")
	class := flag.Int64("class-id", 0, "class ID")
	flag.Parse()
	if *mode != "schema" && *mode != "fresh" && *mode != "class" && *mode != "user" {
		return fmt.Errorf("explicit admin mode required")
	}
	c, e := config.Load(os.Getenv)
	if e != nil {
		return e
	}
	db, e := repository.Open(c)
	if e != nil {
		return fmt.Errorf("target connection failed")
	}
	defer db.Close()
	ctx := context.Background()
	switch *mode {
	case "schema":
		if e = repository.PrepareSchema(ctx, db); e != nil {
			return e
		}
		fmt.Println("schema version 1 ready")
	case "fresh":
		r, e := provision.Fresh(ctx, db, c.UploadRoot, os.Getenv)
		if e != nil {
			return e
		}
		return json.NewEncoder(os.Stdout).Encode(r)
	case "class":
		id, e := provision.CreateClass(ctx, db, *name)
		if e != nil {
			return e
		}
		fmt.Printf("class id: %d\n", id)
	case "user":
		id, e := provision.CreateUser(ctx, db, *name, os.Getenv("ACCOUNT_PASSWORD"), *role, *class)
		if e != nil {
			return e
		}
		fmt.Printf("user id: %d\n", id)
	}
	return nil
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
