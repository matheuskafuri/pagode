package main

// Module describes an optional feature that can be removed from the project.
type Module struct {
	Name        string   // Display name in TUI (e.g., "Payment (Stripe)")
	Description string   // One-liner description
	Files       []string // Standalone files to delete
	Dirs        []string // Directories to delete recursively
	EntSchemas  []string // Ent schema files to remove (triggers ent-gen)
	UserEdges   []string // Edge names to remove from ent/schema/user.go
	NavItems    []string // Sidebar nav item titles to remove from AppSidebar.tsx
	RouteNames  []string // Constants to remove from routenames/names.go
	ConfigKeys  []string // Top-level keys to remove from config.yaml
	GoDeps      []string // Go module paths to expect removal via go mod tidy
	MakeTargets []string // Makefile targets to remove
}

// Modules returns all optional feature modules.
func Modules() []Module {
	return []Module{
		paymentModule(),
		chatModule(),
		mailModule(),
		tasksModule(),
		filesModule(),
	}
}

func paymentModule() Module {
	return Module{
		Name:        "Payment (Stripe)",
		Description: "Subscription billing, one-time payments, premium access",
		Files: []string{
			"pkg/handlers/billing.go",
			"pkg/handlers/plans.go",
			"pkg/handlers/products.go",
			"pkg/handlers/premium.go",
			"pkg/services/payment.go",
			"pkg/services/payment_stripe.go",
			"resources/js/Pages/Billing.tsx",
			"resources/js/Pages/Plans.tsx",
			"resources/js/Pages/Products.tsx",
			"resources/js/Pages/Premium.tsx",
			"resources/js/components/PaymentForm.tsx",
			"resources/js/types/stripe.d.ts",
		},
		EntSchemas: []string{
			"ent/schema/paymentcustomer.go",
			"ent/schema/paymentmethod.go",
			"ent/schema/paymentintent.go",
			"ent/schema/subscription.go",
		},
		UserEdges:  []string{"payment_customer"},
		NavItems:   []string{"Plans", "Products", "Premium", "Billing"},
		RouteNames: []string{"Plans", "PlansSubscribe", "Products", "ProductsPurchase", "Premium", "Billing", "BillingCancel"},
		ConfigKeys: []string{"payment"},
		GoDeps:     []string{"github.com/stripe/stripe-go/v82"},
	}
}

func chatModule() Module {
	return Module{
		Name:        "Chat (WebSocket)",
		Description: "Real-time community chat with rooms, voice messages",
		Files: []string{
			"pkg/handlers/chat.go",
			"resources/js/hooks/useChat.ts",
			"resources/js/types/chat.d.ts",
			"resources/js/hooks/useAudioRecorder.ts",
		},
		Dirs: []string{
			"static/chat-uploads",
			"resources/js/Pages/Chat",
			"resources/js/components/chat",
			"pkg/chat",
		},
		EntSchemas: []string{
			"ent/schema/chatroom.go",
			"ent/schema/chatmessage.go",
			"ent/schema/chatban.go",
		},
		UserEdges:   []string{"owned_chat_rooms", "chat_messages", "chat_bans", "chat_bans_issued"},
		NavItems:    []string{"Chat"},
		RouteNames:  []string{"ChatRooms", "ChatRoomCreate", "ChatRoom", "ChatWebSocket", "ChatBanUser", "ChatUnbanUser", "ChatDeleteRoom"},
		ConfigKeys:  []string{"chat"},
		MakeTargets: []string{"chat-clear"},
	}
}

func mailModule() Module {
	return Module{
		Name:        "Mail (Resend)",
		Description: "Transactional emails (verification, password reset, contact)",
		Files: []string{
			"pkg/handlers/contact.go",
			"pkg/services/mail.go",
			"pkg/services/mail_test.go",
			"pkg/ui/forms/contact.go",
			"pkg/ui/pages/contact.go",
			"pkg/ui/emails/auth.go",
		},
		ConfigKeys: []string{"mail"},
		RouteNames: []string{"Contact", "ContactSubmit"},
		GoDeps:     []string{"github.com/resend/resend-go/v2"},
	}
}

func tasksModule() Module {
	return Module{
		Name:        "Background Tasks",
		Description: "Async job queue with admin monitoring UI",
		Files: []string{
			"pkg/handlers/task.go",
			"pkg/tasks/example.go",
			"pkg/tasks/register.go",
			"pkg/ui/forms/task.go",
			"pkg/ui/pages/task.go",
		},
		Dirs:       []string{"pkg/tasks"},
		RouteNames: []string{"Task", "TaskSubmit", "AdminTasks"},
		ConfigKeys: []string{"tasks"},
		GoDeps:     []string{"github.com/mikestefanello/backlite"},
	}
}

func filesModule() Module {
	return Module{
		Name:        "File Upload",
		Description: "File upload handling with filesystem abstraction",
		Files: []string{
			"pkg/handlers/files.go",
			"resources/js/Pages/UploadFile.tsx",
			"pkg/ui/forms/file.go",
			"pkg/ui/pages/file.go",
			"pkg/ui/models/file.go",
		},
		NavItems:   []string{"Upload Files"},
		ConfigKeys: []string{"files"},
		RouteNames: []string{"Files", "FilesSubmit"},
		GoDeps:     []string{"github.com/spf13/afero"},
	}
}
