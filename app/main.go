package main

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"os"
	"strconv"
	"strings"
)

const (
	OpNewTask         = "new_task"
	OpWalkDog         = "walk_dog"
	OpBalance         = "balance"
	OpHistory         = "history"
	OpGetMoney        = "get_money"
	OpFinishTask      = "finish_task"
	OpFreeDish        = "free_dish"
	OpDirtyDish       = "dirty_dish"
	OpGoToShop        = "go_to_shop"
	OpWashFloorInFlat = "wash_floor_in_flat"
	OpChild           = "child"
	OpParent          = "parent"
)

//var mainKeyboard = tgbotapi.NewInlineKeyboardMarkup(
//	tgbotapi.NewInlineKeyboardRow(
//		tgbotapi.NewInlineKeyboardButtonData("Новое Задание", OpNewTask)),
//	tgbotapi.NewInlineKeyboardRow(
//		tgbotapi.NewInlineKeyboardButtonData("Завершить Текущее Задание", OpFinishTask)),
//	tgbotapi.NewInlineKeyboardRow(
//		tgbotapi.NewInlineKeyboardButtonData(addCoinToString("Баланс динокоинов"), OpBalance)),
//
//	tgbotapi.NewInlineKeyboardRow(
//		tgbotapi.NewInlineKeyboardButtonData("История заданий", OpHistory)),
//
//	tgbotapi.NewInlineKeyboardRow(
//		tgbotapi.NewInlineKeyboardButtonData("Получить деньги", OpGetMoney)),
//)

var registerKeyBoard = tgbotapi.NewReplyKeyboard(
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("Зарегистрироваться")),
)

var parentKeyBoard = tgbotapi.NewReplyKeyboard(
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("Добавить ребенка")),
)

var mainKeyboard = tgbotapi.NewReplyKeyboard(
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("Новое Задание"),
	),
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("Баланс динокоинов"),
		tgbotapi.NewKeyboardButton("Завершить Текущее Задание"),
	),

	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("История заданий"),
		tgbotapi.NewKeyboardButton("Получить деньги"),
	),
)

var childAdultKeyboard = tgbotapi.NewInlineKeyboardMarkup(
	tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Ребенок", OpChild),
		tgbotapi.NewInlineKeyboardButtonData("Взрослый", OpParent),
	))

var tasksKeyboard = tgbotapi.NewInlineKeyboardMarkup(
	tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Погулять с собакой", OpWalkDog)),
	tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Разгрузить посудомойку", OpFreeDish)),

	tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Загрузить посудомойку", OpDirtyDish)),

	tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Сходить в магазин", OpGoToShop)),
	tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Помыть полы в квартире", OpWashFloorInFlat)),
)

func main() {
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_APITOKEN"))
	if err != nil {
		os.Exit(1)
	}

	db, err := NewBoltDB("dinocoins.db")
	if err != nil {
		os.Exit(1)
	}

	bot.Debug = false

	// Create a new UpdateConfig struct with an offset of 0. Offsets are used
	// to make sure Telegram knows we've handled previous values and we don't
	// need them repeated.
	updateConfig := tgbotapi.NewUpdate(0)

	// Tell Telegram we should wait up to 30 seconds on each request for an
	// update. This way we can get information just as quickly as making many
	// frequent requests without having to send nearly as many.
	updateConfig.Timeout = 10

	// Start polling Telegram for updates.
	updates := bot.GetUpdatesChan(updateConfig)

	// Loop through each update.
	for update := range updates {
		// Check if we've gotten a message update.
		if update.Message != nil {
			// Construct a new message from the given chat ID and containing
			// the text that we received.
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, update.Message.Text)

			// If the message was open, add a copy of our numeric keyboard.
			switch update.Message.Text {
			case "Зарегистрироваться":
				msg.ReplyMarkup = childAdultKeyboard
			case "Новое Задание":
				msg.ReplyMarkup = tasksKeyboard
			case "Баланс динокоинов":
				db.Balance(update.Message.From.ID)
			case "Завершить Текущее Задание":
				msg.Text = "Запрос отправлен родителям. Жди подтверждения"
			case "История заданий":
				transactions, e := db.ShowLastNTransactions(update.Message.From.ID, 30)
				if e != nil {
					log.Printf("Unable to show last 10 transactions %+v", e)
					msg.Text = "Ошибка"
				} else {
					var str string
					for _, t := range transactions {
						str = strings.Join([]string{str, t.Operation, t.Timestamp.Format(TSNano), t.Status, strconv.Itoa(t.Cost), "\n"}, "\t")
					}

					if str == "" {
						str = "test"
					}

					msg.Text = str
				}
			default:
				isRegistered, e := db.CheckRegistered(update.Message.From.ID)

				if e != nil {
					log.Printf("[ERROR] %+v", e)
					msg.Text = "Ошибка"
				} else {
					if isRegistered {
						user, e := db.FindUser(update.Message.From.ID)
						if e != nil {
							log.Printf("[ERROR] %+v", e)
							msg.Text = "Ошибка"
						} else {
							if user.Type == PARENT {
								msg.ReplyMarkup = parentKeyBoard
							} else {
								msg.ReplyMarkup = mainKeyboard
							}
						}
					} else {
						msg.ReplyMarkup = registerKeyBoard
					}
				}
			}

			// Send the message.
			if _, err = bot.Send(msg); err != nil {
				panic(err)
			}
		} else if update.CallbackQuery != nil {
			msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "test")

			switch update.CallbackData() {
			case OpNewTask:
				msg.ReplyMarkup = tasksKeyboard
			case OpWalkDog, OpFreeDish, OpDirtyDish, OpGoToShop, OpWashFloorInFlat:
				_, e := db.CreateTransaction(update.CallbackData(), update.CallbackQuery.From.ID)
				if e != nil {
					log.Printf("Unable to create transaction %+v", e)
				}

			case OpParent:
				user := User{
					ID:       update.CallbackQuery.From.ID,
					Nickname: update.CallbackQuery.From.UserName,
					Type:     PARENT,
				}
				e := db.RegisterUser(user)
				if e != nil {
					log.Println("[ERROR] %+v", e)
				} else {
					msg.ReplyMarkup = parentKeyBoard
				}
			case OpChild:
				hasParent, e := db.HasParent(update.CallbackQuery.From.ID)
				if e != nil {
					log.Println("[ERROR] %+v", e)
				}
				if hasParent {
					user := User{
						ID:       update.CallbackQuery.From.ID,
						Nickname: update.CallbackQuery.From.UserName,
						Type:     CHILD,
					}

					e := db.RegisterUser(user)
					if e != nil {
						log.Println("[ERROR] %+v", e)
					} else {
						msg.ReplyMarkup = mainKeyboard
					}
				} else {
					msg.Text = "Попроси сначала зарегистрироваться родителя. Отправь ему никнейм Дино @dinocoins_bot"
				}

			default:
				log.Println("[WARN] unknown operation %s", update.CallbackData())
			}
			// Respond to the callback query, telling Telegram to show the user
			// a message with the data received.
			callback := tgbotapi.NewCallback(update.CallbackQuery.ID, update.CallbackQuery.Data)
			if _, err := bot.Request(callback); err != nil {
				panic(err)
			}

			// And finally, send a message containing the data received.
			if _, err := bot.Send(msg); err != nil {
				panic(err)
			}
		}
	}

}

func addCoinToString(s string) string {
	var sb strings.Builder
	sb.WriteString(s)
	sb.WriteRune('\U0001FA99')
	return sb.String()
}
