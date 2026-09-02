# 📬 Background Queue & Workers Guide

`padi_go_core/queue` provides a robust, database-backed background job queue with delayed dispatching, automatic retries, and type-safe payload deserialization.

---

## 🏗️ Registering Type-Safe Job Handlers

```go
package jobs

import (
	"fmt"
	"github.com/wibiesana/padi_go_core/email"
	"github.com/wibiesana/padi_go_core/queue"
)

type SendInvoiceJob struct {
	OrderID uint   `json:"order_id"`
	Email   string `json:"email"`
	PDFPath string `json:"pdf_path"`
}

func init() {
	// Register typed handler (automatically parses JSON payload into SendInvoiceJob)
	queue.RegisterTyped("SendInvoice", func(job SendInvoiceJob) error {
		return email.SendHTML(
			job.Email,
			fmt.Sprintf("Your Invoice #%d", job.OrderID),
			"<h1>Invoice Attached</h1>",
			job.PDFPath,
		)
	})
}
```

---

## 🚀 Dispatching Jobs

### 1. Immediate Dispatch
```go
queue.Push("SendInvoice", SendInvoiceJob{
    OrderID: 1042,
    Email:   "customer@example.com",
    PDFPath: "storage/invoices/inv_1042.pdf",
}, "emails")
```

### 2. Delayed Dispatch (`Later` / `PushLater`)
```go
// Run job after 15 minutes
queue.Later(15*time.Minute, "SendInvoice", SendInvoiceJob{
    OrderID: 1042,
    Email:   "customer@example.com",
}, "emails")
```

---

## 📊 Queue Stats & Management

```go
// Check pending job count
count, err := queue.Size("emails")

// Clear all pending jobs in a queue
err = queue.Clear("emails")
```

---

## 👷 Running Workers

In a CLI runner or background goroutine:

```go
// 1. Process 10 jobs then exit
queue.Work("emails", 10)

// 2. Process continuously with graceful shutdown context
ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer cancel()

queue.WorkWithContext(ctx, "emails", 0) // 0 = daemon mode
```
