package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
)

// Testing
// curl -H "Content-Type: application/json" -X POST -d '{"bucket":"food","keystore":{"username":"xyz","password":"xyz"}}' 'http://zack:123@localhost:8080/'

// Payload is how the data is entered and returned from the BoltDB server
// DB is the name of the database file
// Bucket is the name of the bucket in the database
// Keystore is a map of the keys and values
type Payload struct {
	DB       string            `json:"db" binding:"required"`
	Bucket   string            `json:"bucket" binding:"required"`
	Keystore map[string]string `json:"keystore" binding:"required"`
}

func updateDatabase(dbname string, bucket string, keystore map[string]string) (string, bool) {
	db, err := bolt.Open(dbname+".db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	err = db.Update(func(tx *bolt.Tx) error {
		b, err2 := tx.CreateBucketIfNotExists([]byte(bucket))
		if err2 != nil {
			return err2
		}
		for key, value := range keystore {
			err2 := b.Put([]byte(key), []byte(value))
			if err2 != nil {
				return err
			}
		}
		return err
	})
	if err != nil {
		return fmt.Sprintf("Error: '%s'", err.Error()), false
	}

	return fmt.Sprintf("Updated %d keys in %s", len(keystore), bucket), true
}

func getFromDatabase(dbname string, bucket string, keys map[string]string) (string, bool, map[string]string) {
	keystore := make(map[string]string)

	db, err := bolt.Open(dbname+".db", 0600, nil)
	if err != nil {
		return fmt.Sprintf("Error: '%s'", err.Error()), false, keystore
	}
	defer db.Close()

	if len(keys) == 0 {
		// Get all keys
		err = db.View(func(tx *bolt.Tx) error {
			// Assume bucket exists and has keys
			b := tx.Bucket([]byte(bucket))
			if b == nil {
				return errors.New("Bucket does not exist")
			}
			fmt.Println(b)
			c := b.Cursor()
			for k, v := c.First(); k != nil; k, v = c.Next() {
				keystore[string(k)] = string(v)
			}
			return nil
		})
	} else {
		// Get specified keys
		err = db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(bucket))
			if b == nil {
				return errors.New("Bucket does not exist")
			}
			for key := range keys {
				v := b.Get([]byte(key))
				if v != nil {
					keystore[key] = string(v)
				}
			}
			return nil
		})
	}
	if err != nil {
		return fmt.Sprintf("Error: '%s'", err.Error()), false, keystore
	}
	return fmt.Sprintf("Got %d keys in %s", len(keystore), bucket), true, keystore
}

func deleteFromDatabase(dbname string, bucket string, keys map[string]string) (string, bool) {
	db, err := bolt.Open(dbname+".db", 0600, nil)
	if err != nil {
		return fmt.Sprintf("Error: '%s'", err.Error()), false
	}
	defer db.Close()

	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return errors.New("Bucket does not exist")
		}
		for key := range keys {
			b.Delete([]byte(key))
		}
		return err
	})
	if err != nil {
		return fmt.Sprintf("Error: '%s'", err.Error()), false
	}
	return fmt.Sprintf("Deleted %d keys in %s", len(keys), bucket), true
}

func handleRequests(c *gin.Context) {
	username, password, _ := c.Request.BasicAuth()
	if username != SpecifiedUsername || password != SpecifiedPassword {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "Incorrent credentials",
		})
		return
	}
	var json Payload
	if c.BindJSON(&json) == nil {
		message := "Incorrect method"
		success := false
		if c.Request.Method == "POST" {
			message, success = updateDatabase(json.DB, json.Bucket, json.Keystore)
		} else if c.Request.Method == "GET" {
			message, success, json.Keystore = getFromDatabase(json.DB, json.Bucket, json.Keystore)
		} else if c.Request.Method == "DELETE" {
			message, success = deleteFromDatabase(json.DB, json.Bucket, json.Keystore)
		}
		c.JSON(http.StatusOK, gin.H{
			"success":  success,
			"message":  message,
			"keystore": json.Keystore,
		})
	} else {
		c.JSON(http.StatusNotAcceptable, gin.H{
			"success":  false,
			"message":  "Cannot bind JSON",
			"keystore": json.Keystore,
		})
	}
}

var Port, SpecifiedUsername, SpecifiedPassword string

func main() {
	flag.StringVar(&SpecifiedUsername, "user", RandStringBytesMaskImprSrc(4), "port to use for server")
	flag.StringVar(&SpecifiedPassword, "pass", RandStringBytesMaskImprSrc(4), "port to use for server")
	flag.StringVar(&Port, "port", "8080", "port to use for server")
	flag.Parse()
	r := gin.Default()
	r.GET("/v1", handleRequests)    // Get keys from BoltDB
	r.POST("/v1", handleRequests)   // Post keys to BoltDB
	r.DELETE("/v1", handleRequests) // Delete keys in BoltDB

	log.Printf("Listening on 0.0.0.0:%s\n", Port)
	log.Printf("Authenticated with user: %s and pw: %s\n", SpecifiedUsername, SpecifiedPassword)
	r.Run(":" + Port) // listen and serve on 0.0.0.0:8080
}