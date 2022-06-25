package main

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"main/store"
	"os"
	"strconv"
	"strings"
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
		tgbotapi.NewKeyboardButton("Добавить ребенка"),
		tgbotapi.NewKeyboardButton("Отсоединить ребенка")),
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("Редактировать стоимость"),
		tgbotapi.NewKeyboardButton("Подтвердить задание")),
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
		tgbotapi.NewInlineKeyboardButtonData("Ребенок", store.OpChild),
		tgbotapi.NewInlineKeyboardButtonData("Взрослый", store.OpParent),
	))

var tasksKeyboard = tgbotapi.NewInlineKeyboardMarkup(
	tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Погулять с собакой", store.OpWalkDog)),
	tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Разгрузить посудомойку", store.OpFreeDish)),

	tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Загрузить посудомойку", store.OpDirtyDish)),

	tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Сходить в магазин", store.OpGoToShop)),
	tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Помыть полы в квартире", store.OpWashFloorInFlat)),
)

var taskCancelationKeyboard = tgbotapi.NewInlineKeyboardMarkup(
	tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Отмениить текущее задание", store.OpCancelCurrentTask),
		tgbotapi.NewInlineKeyboardButtonData("Завершить текущее задание", store.OpFinishCurrentTask),
	))

func main() {
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_APITOKEN"))
	if err != nil {
		os.Exit(1)
	}

	db, err := store.NewBoltDB("dinocoins.db")
	if err != nil {
		os.Exit(1)
	}

	am, err := store.NewActionManager(db)
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

			// Child add process
			// TODO: add additional flat to prevent accidental input with @ prefix
			if strings.HasPrefix(update.Message.Text, "@") {
				err := db.BindChildToParent(update.Message.From.ID, update.Message.Text)
				if err != nil {
					log.Printf("Unable to bind a child to parent %+v", err)
					msg.Text = "Ошибка"
				}
			}

			if strings.Contains(update.Message.Text, "запрос на подтверждение задания") {
				log.Printf("Accept or Decline")
			}

			// If the message was open, add a copy of our numeric keyboard.
			switch update.Message.Text {
			case "Зарегистрироваться":
				msg.ReplyMarkup = childAdultKeyboard
			case "Новое Задание":
				hasOpenTask, err := db.HasOpenTask(update.Message.From.ID)
				if err != nil {
					log.Printf("Unable to find open task %+v", err)
					msg.Text = "Ошибка"
				}

				if hasOpenTask {
					msg.Text = "У тебя есть незавершенное задание. Сначала заверши его или отмени"
					msg.ReplyMarkup = taskCancelationKeyboard
				} else {
					msg.ReplyMarkup = tasksKeyboard
				}

			case "Баланс динокоинов":
				balance, _ := db.Balance(update.Message.From.ID)
				msg.Text = strconv.Itoa(balance) + " dinocoins"
			case "Завершить Текущее Задание":
				childId := update.Message.From.ID
				childName := update.Message.From.UserName
				op, parentChatid, err := am.SendRequestToCompleteCurrentTask(childId, childName)
				if err != nil {
					log.Printf("Unable to complete task %+v", err)
					msg.Text = "Ошибка. Невозможно завершить задание"
				} else {
					msgForParent := tgbotapi.NewMessage(parentChatid,
						"Ребенок с ником "+childName+" отправил запрос на подтверждение задания "+op)
					if _, err = bot.Send(msgForParent); err != nil {
						panic(err)
					}
					msg.Text = "Запрос отправлен родителям. Жди подтверждения"
				}
			case "История заданий":
				transactions, e := db.ShowLastNTransactions(update.Message.From.ID, 30)
				if e != nil {
					log.Printf("Unable to show last 10 transactions %+v", e)
					msg.Text = "Ошибка"
				} else {
					var str string
					for _, t := range transactions {
						str = strings.Join([]string{str, t.Operation, t.Timestamp.Format(store.TSNano), t.Status, strconv.Itoa(t.Cost), "\n"}, "\t")
					}

					if str == "" {
						str = "test"
					}

					msg.Text = str
				}

			case "Добавить ребенка":
				msg.Text = "Пришли никнейм ребенка (@test)"
			case "Подтвердить задание":
				// find list of children and build kbd
				children, _ := db.FindChildren(update.Message.From.ID)
				kbdWithChildrenList := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow())

				for _, child := range children {
					kbdWithChildrenList.InlineKeyboard[0] = append(kbdWithChildrenList.InlineKeyboard[0],
						tgbotapi.NewInlineKeyboardButtonData(child, "cmd"+child))
				}

				msg.Text = "Выбери ребенка, которому нужно подтвердить задание"
				msg.ReplyMarkup = kbdWithChildrenList
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
							if user.Type == store.PARENT {
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
			case store.OpNewTask:
				msg.ReplyMarkup = tasksKeyboard
			case store.OpWalkDog, store.OpFreeDish, store.OpDirtyDish, store.OpGoToShop, store.OpWashFloorInFlat:
				_, e := db.CreateTransaction(update.CallbackData(), update.CallbackQuery.From.ID)
				if e != nil {
					log.Printf("Unable to create transaction %+v", e)
				}

			case store.OpParent:
				user := store.User{
					ID:       update.CallbackQuery.From.ID,
					ChatID:   update.CallbackQuery.Message.Chat.ID,
					Nickname: update.CallbackQuery.From.UserName,
					Type:     store.PARENT,
				}
				e := db.RegisterUser(user)
				if e != nil {
					log.Println("[ERROR] %+v", e)
				} else {
					msg.ReplyMarkup = parentKeyBoard
				}
			case store.OpChild:
				hasParent, e := db.HasParent(update.CallbackQuery.From.UserName)
				if e != nil {
					log.Println("[ERROR] %+v", e)
				}
				if hasParent {
					user := store.User{
						ID:       update.CallbackQuery.From.ID,
						ChatID:   update.CallbackQuery.Message.Chat.ID,
						Nickname: update.CallbackQuery.From.UserName,
						Type:     store.CHILD,
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

			case store.OpCancelCurrentTask:
				err := db.CancelCurrentTask(update.CallbackQuery.From.ID)
				if err != nil {
					log.Printf("[ERROR] %+v", err)
					msg.Text = "Ошибка"
				} else {
					msg.Text = "Текущее задание отменено"
				}

			case store.OpFinishCurrentTask:
				// find parent
				// find current transaction
				// send to parent message: I want to finish task blah-bla
				msg.Text = "Запрос отправлен родителям. Жди подтверждения"
				parentId, err := db.FindParentIdByChildNickName(update.CallbackQuery.From.UserName)
				if err != nil {
					log.Printf("[ERROR] %+v", err)
					msg.Text = "Ошибка"
				} else {
					parent, err := db.FindUser(parentId)
					if err != nil {
						log.Printf("[ERROR] %+v", err)
						msg.Text = "Ошибка"
					} else {
						t, err := db.GetCurrentTransaction(update.CallbackQuery.From.ID)
						if err != nil {
							log.Printf("[ERROR] %+v", err)
							msg.Text = "Ошибка"
						} else {
							text := fmt.Sprintf("%s закончил задание %s. Подтвердите",
								update.CallbackQuery.From.UserName, t.Operation)
							msgForParent := tgbotapi.NewMessage(parent.ChatID, text)
							if _, err := bot.Send(msgForParent); err != nil {
								panic(err)
							}
						}
					}
				}
			default:
				if strings.Contains(update.CallbackData(), "cmd@") {
					childNickName := update.CallbackData()[4:]
					childUser, err := db.FindUserByNickname(childNickName)
					if childUser.ID == 0 {
						log.Printf("Unable to find a child with nickname %s %+v", childNickName, err)
					} else {
						err = am.ConfirmTransaction(childUser.ID)
						if err != nil {
							log.Printf("Unable to confirm transaction %+v", err)
						}
					}
				} else {
					log.Println("[WARN] unknown operation %s", update.CallbackData())
				}
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
