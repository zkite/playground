package db

import (
	"database/sql"
	"fmt"
	"hello/response"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func CreateTables(db *sql.DB) error {
	createClientTable := `
        CREATE TABLE IF NOT EXISTS client(
        name TEXT NOT NULL UNIQUE,
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
        job_type TEXT DEFAULT 'common'
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

// name=udpu["subscriber_uid"],
// udpu_upstream_qos=udpu["upstream_qos"],
// udpu_downstream_qos=udpu["downstream_qos"],
// udpu_hostname=udpu["hostname"],
// udpu_location=udpu["location"],
// udpu_role=udpu["role"],

func InsertIntoClient(db *sql.DB, r *response.ServerResponse) error {
	// SQL-запрос для вставки данных
	query := `INSERT INTO client (name, boot_status, udpu_upstream_qos, udpu_downstream_qos, udpu_hostname, udpu_location, udpu_role, client_frequency) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	// Выполнение запроса
	_, err := db.Exec(query, r.SubscriberUID, "first_boot", r.UpstreamQoS, r.DownstreamQoS, r.Hostname, r.Location, r.Role, "")
	if err != nil {
		return err
	}

	return nil
}

type Job struct {
	name           string
	command        string
	frequency      string
	locked         string
	require_output string
	job_type       string
}

type Queue struct {
	name           string
	description    string
	queue          string
	role           string
	require_output string
	frequency      string
	locked         string
}

type Client struct {
	subscriber_uid      string
	client_frequency    string
	boot_status         string
	udpu_upstream_qos   string
	udpu_downstream_qos string
	udpu_hostname       string
	udpu_location       string
	udpu_role           string
}

func GetData(db *sql.DB, table string, name string) interface{} {
	stmt := fmt.Sprintf("SELECT * FROM %s WHERE name = ?", table)

	rows, err := db.Query(stmt, name)
	if err != nil {
		log.Printf("Error querying data from %s: %v", table, err)
		return nil
	}
	defer rows.Close()

	switch table {
	case "job":
		var jobs []Job
		for rows.Next() {
			var j Job
			if err := rows.Scan(&j.name, &j.command, &j.frequency, &j.locked, &j.require_output, &j.job_type); err != nil {
				log.Println("Error scanning Job row:", err)
				continue
			}
			jobs = append(jobs, j)
		}
		if len(jobs) == 0 {
			return nil
		}
		return jobs
	case "queue":
		var queues []Queue
		for rows.Next() {
			var q Queue
			if err := rows.Scan(&q.name, &q.description, &q.queue, &q.role, &q.require_output, &q.frequency, &q.locked); err != nil {
				log.Println("Error scanning Queue row:", err)
				continue
			}
			queues = append(queues, q)
		}
		if len(queues) == 0 {
			return nil
		}
		return queues
	case "client":
		var client []Client
		for rows.Next() {
			var c Client
			if err := rows.Scan(&c.subscriber_uid, &c.client_frequency, &c.boot_status, &c.udpu_upstream_qos, &c.udpu_downstream_qos, &c.udpu_hostname, &c.udpu_location, &c.udpu_role); err != nil {
				log.Println("Error scanning Queue row:", err)
				continue
			}
			client = append(client, c)
		}
		if len(client) == 0 {
			return nil
		}
		return client
	}

	return nil
}

func UpdateData(db *sql.DB, table string, name string, fields map[string]interface{}) {
	if len(fields) == 0 {
		return
	}

	// Constructing the SET part of the SQL statement
	setParts := ""
	args := make([]interface{}, 0, len(fields)+1)
	for k, v := range fields {
		if setParts != "" {
			setParts += ", "
		}
		setParts += fmt.Sprintf("%s = ?", k)
		args = append(args, v)
	}

	stmt := fmt.Sprintf("UPDATE %s SET %s WHERE name = ?", table, setParts)
	args = append(args, name)

	_, err := db.Exec(stmt, args...)
	if err != nil {
		log.Printf("Can't update %s with data: %v", table, err)
	}
}

func DeleteData(db *sql.DB, table string, name string) {
	stmt := fmt.Sprintf("DELETE FROM %s WHERE name = ?", table)
	_, err := db.Exec(stmt, name)
	if err != nil {
		log.Printf("Can't delete %s: %s, error: %v", table, name, err)
	}
}

func GetWithFrequency(db *sql.DB, table string, frequency string) []map[string]interface{} {
	if table != "job" && table != "queue" {
		log.Println("Table should be either job or queue")
		return nil
	}

	stmt := fmt.Sprintf("SELECT * FROM %s WHERE frequency = ?", table)
	rows, err := db.Query(stmt, frequency)
	if err != nil {
		log.Printf("Can't fetch %ss with frequency: %s", table, frequency)
		return nil
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		log.Println("Failed to get columns:", err)
		return nil
	}

	var results []map[string]interface{}

	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for rows.Next() {
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			log.Println("Failed to scan row:", err)
			continue
		}

		entry := make(map[string]interface{})
		for i, col := range columns {
			var val interface{}
			val = values[i]
			b, ok := val.([]byte)
			if ok {
				val = string(b)
			}
			entry[col] = val
		}

		results = append(results, entry)
	}

	return results
}

func GetScheduled(db *sql.DB, table string) []map[string]interface{} {
	return GetWithFrequency(db, table, "1 OR frequency = '15' OR frequency = '60' OR frequency = '1440'")
}

func getEveryBootJobs(db *sql.DB) []map[string]interface{} {
	results := GetWithFrequency(db, "job", "every_boot")
	if results == nil {
		log.Println("Can't get job's data with 'every_boot' frequency")
	}
	return results
}

func getFirstBootJobs(db *sql.DB) []map[string]interface{} {
	results := GetWithFrequency(db, "job", "first_boot")
	if results == nil {
		log.Println("Can't get job's data with 'first_boot' frequency")
	}
	return results
}

func getJobOnce(db *sql.DB) []map[string]interface{} {
	results := GetWithFrequency(db, "job", "once")
	if results == nil {
		log.Println("Can't get job's data with 'once' frequency")
	}
	return results
}

func getJobsScheduled(db *sql.DB) []map[string]interface{} {
	results := GetScheduled(db, "job")
	if results == nil {
		log.Println("Get scheduled jobs: error")
	}
	return results
}

func getQueuesScheduled(db *sql.DB) []map[string]interface{} {
	results := GetScheduled(db, "queue")
	if results == nil {
		log.Println("Get scheduled queues: error")
	}
	return results
}
