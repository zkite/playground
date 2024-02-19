package db

import (
	"database/sql"
	"hello/response"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func CreateTables(db *sql.DB) error {
	createClientTable := `
        CREATE TABLE IF NOT EXISTS client(
        subscriber_uid TEXT NOT NULL UNIQUE,
        client_frequency TEXT,
        boot_status TEXT DEFAULT 'first_boot',
        udpu_upstream_qos TEXT,
        udpu_downstream_qos TEXT,
        udpu_hostname TEXT,
        udpu_location TEXT,
        udpu_role TEXT
        );
    `
	if _, err := db.Exec(createClientTable); err != nil {
		log.Printf("Can't create client table: %v", err)
		return err
	}

	createJobTable := `
        CREATE TABLE IF NOT EXISTS job(
        name TEXT NOT NULL UNIQUE,
        command TEXT,
        frequency TEXT,
        locked TEXT DEFAULT 'false',
        require_output TEXT DEFAULT 'false',
        required_software TEXT DEFAULT '',
        type TEXT DEFAULT 'common'
        );
    `
	if _, err := db.Exec(createJobTable); err != nil {
		log.Printf("Can't create job table: %v", err)
		return err
	}

	createQueueTable := `
        CREATE TABLE IF NOT EXISTS queue(
        name TEXT NOT NULL UNIQUE,
        description TEXT,
        queue TEXT,
        role TEXT,
        require_output TEXT DEFAULT 'false',
        frequency TEXT DEFAULT 'every_boot',
        locked TEXT DEFAULT 'false'
        );
    `
	if _, err := db.Exec(createQueueTable); err != nil {
		log.Printf("Can't create queue table: %v", err)
		return err
	}

	// Все таблицы успешно созданы, возвращаем nil
	return nil
}

func InsertIntoClient(db *sql.DB, r *response.ServerResponse) error {
	// SQL-запрос для вставки данных
	query := `INSERT INTO client (subscriber_uid, client_frequency, boot_status, udpu_upstream_qos, udpu_downstream_qos, udpu_hostname, udpu_location, udpu_role) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	// Выполнение запроса
	_, err := db.Exec(query, r.SubscriberUID, r.Location, "first_boot", r.UpstreamQoS, r.DownstreamQoS, r.Hostname, r.Location, r.Role)
	if err != nil {
		return err
	}

	return nil
}
