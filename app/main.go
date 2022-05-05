package main

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"os"
	"strings"
)

const (
	OpNewTask         = "new_task"
	OpWalkDog         = "walk_dog"
	OpBalance         = "balance"
	OpHistory         = "history"
	OpGetMoney        = "get_money"
	OpFreeDish        = "free_dish"
	OpDirtyDish       = "dirty_dish"
	OpGoToShop        = "go_to_shop"
	OpWashFloorInFlat = "wash_floor_in_flat"
)

var mainKeyboard = tgbotapi.NewInlineKeyboardMarkup(
	tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Новое Задание", OpNewTask)),
	tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(addCoinToString("Баланс динокоинов"), OpBalance)),

	tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("История заданий", OpHistory)),

	tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Получить деньги", OpGetMoney)),
)

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

	bot.Debug = true

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
			case "Open":
				msg.ReplyMarkup = mainKeyboard
			}

			// Send the message.
			if _, err = bot.Send(msg); err != nil {
				panic(err)
			}
		} else if update.CallbackQuery != nil {
			msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "")

			switch update.CallbackData() {
			case OpNewTask:
				msg.ReplyMarkup = tasksKeyboard
			case OpWalkDog, OpFreeDish, OpDirtyDish, OpGoToShop, OpWashFloorInFlat:
				_, e := db.CreateTransaction(update.CallbackData(), update.Message.From.ID)
				if e != nil {
					log.Printf("Unable to create transaction %+v", e)
				}
			case OpBalance:
				db.Balance(update.Message.From.ID)

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
