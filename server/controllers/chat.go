package controllers

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/interfaces"
	"github.com/verbeux-ai/whatsmiau/lib/whatsmiau"
	"github.com/verbeux-ai/whatsmiau/server/dto"
	"github.com/verbeux-ai/whatsmiau/utils"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.uber.org/zap"
)

type Chat struct {
	repo      interfaces.InstanceRepository
	whatsmiau *whatsmiau.Whatsmiau
}

func NewChats(repository interfaces.InstanceRepository, whatsmiau *whatsmiau.Whatsmiau) *Chat {
	return &Chat{
		repo:      repository,
		whatsmiau: whatsmiau,
	}
}

// ReadMessages godoc
// @Summary      Mark messages as read
// @Description  Marks one or more messages as read in a WhatsApp conversation
// @Tags         Chat
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                   true  "Instance ID"
// @Param        body      body      dto.ReadMessagesRequest   true  "Messages to mark as read"
// @Success      200       {object}  map[string]interface{}   "Empty object on success"
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      422       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /instance/{instance}/chat/read-messages [post]
// @Router       /chat/markMessageAsRead/{instance} [post]
func (s *Chat) ReadMessages(ctx echo.Context) error {
	var request dto.ReadMessagesRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}

	result := make(map[string][]string)
	for _, msg := range request.ReadMessages {
		result[msg.RemoteJid] = append(result[msg.RemoteJid], msg.ID)
	}

	for remoteJid, msgs := range result {
		number, err := numberToJid(remoteJid)
		if err != nil {
			zap.L().Error("error converting number to jid", zap.Error(err))
			continue
		}

		if err := s.whatsmiau.ReadMessage(&whatsmiau.ReadMessageRequest{
			MessageIDs: msgs,
			InstanceID: request.InstanceID,
			RemoteJID:  number,
			Sender:     nil,
		}); err != nil {
			zap.L().Error("Whatsmiau.ReadMessages failed", zap.Error(err))
		}
	}

	return ctx.JSON(http.StatusOK, map[string]interface{}{})
}

// SendChatPresence godoc
// @Summary      Send chat presence (typing indicator)
// @Description  Sends a presence status (composing/available) to a WhatsApp contact, with optional auto-stop delay
// @Tags         Chat
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                       true  "Instance ID"
// @Param        body      body      dto.SendChatPresenceRequest  true  "Presence parameters"
// @Success      200       {object}  map[string]interface{}       "Empty object on success"
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      422       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /instance/{instance}/chat/presence [post]
// @Router       /chat/sendPresence/{instance} [post]
func (s *Chat) SendChatPresence(ctx echo.Context) error {
	var request dto.SendChatPresenceRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}

	number, err := numberToJid(request.Number)
	if err != nil {
		zap.L().Error("error converting number to jid", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid number format")
	}

	var presence types.ChatPresence
	switch request.Presence {
	case dto.PresenceComposing:
		presence = types.ChatPresenceComposing
	case dto.PresenceAvailable:
		presence = types.ChatPresencePaused
	}

	presenceType := types.ChatPresenceMediaText
	if request.Type == dto.PresenceTypeAudio {
		presenceType = types.ChatPresenceMediaAudio
	}

	if request.Delay > 0 {
		go func() {
			time.Sleep(time.Duration(request.Delay) * time.Millisecond)
			if err := s.whatsmiau.ChatPresence(&whatsmiau.ChatPresenceRequest{
				InstanceID: request.InstanceID,
				RemoteJID:  number,
				Presence:   types.ChatPresencePaused,
				Media:      types.ChatPresenceMediaText,
			}); err != nil {
				zap.L().Error("Whatsmiau.ReadMessages failed", zap.Error(err))
			}
		}()
	}

	if err := s.whatsmiau.ChatPresence(&whatsmiau.ChatPresenceRequest{
		InstanceID: request.InstanceID,
		RemoteJID:  number,
		Presence:   presence,
		Media:      presenceType,
	}); err != nil {
		zap.L().Error("Whatsmiau.ReadMessages failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "Whatsmiau.ChatPresence failed")
	}

	return ctx.JSON(http.StatusOK, map[string]interface{}{})
}

// GetBase64FromMediaMessage godoc
// @Summary      Download media as base64
// @Description  Downloads and decrypts a media message (image/audio/video/document/sticker)
//
//	using the downloadable fields previously delivered via webhook and returns
//	it as base64-encoded bytes. Response shape mirrors Evolution API's
//	/chat/getBase64FromMediaMessage for drop-in compatibility.
//
// @Tags         Chat
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                        true  "Instance ID"
// @Param        body      body      whatsmiau.DownloadMediaRequest true "Media fields"
// @Success      200       {object}  whatsmiau.DownloadMediaResponse
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      404       {object}  utils.HTTPErrorResponse
// @Failure      422       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /chat/getBase64FromMediaMessage/{instance} [post]
func (s *Chat) GetBase64FromMediaMessage(ctx echo.Context) error {
	instanceID := ctx.Param("instance")
	if instanceID == "" {
		return utils.HTTPFail(ctx, http.StatusBadRequest, nil, "instance ID is required in the URL path")
	}

	var request whatsmiau.DownloadMediaRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}
	request.InstanceID = instanceID

	resp, err := s.whatsmiau.DownloadMedia(ctx.Request().Context(), &request)
	if err != nil {
		// Client-side issues (malformed payload, bad base64, no media fields)
		// map to 4xx. Genuine server/decryption failures stay 500.
		switch {
		case errors.Is(err, whatsmiau.ErrMediaFieldsMissing),
			errors.Is(err, whatsmiau.ErrInvalidBase64):
			return utils.HTTPFail(ctx, http.StatusBadRequest, err, err.Error())
		case errors.Is(err, whatsmiau.ErrInstanceNotFound):
			return utils.HTTPFail(ctx, http.StatusNotFound, err, err.Error())
		default:
			zap.L().Error("Whatsmiau.DownloadMedia failed", zap.Error(err))
			return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to download media")
		}
	}

	return ctx.JSON(http.StatusOK, resp)
}

// FindChats godoc
// @Summary      List chats with stored messages
// @Description  Evolution API v2 compatible endpoint. Lists remoteJIDs that have messages persisted locally (message store) — used by CartZap's backfill (sync:mensagens) to reconstruct a window of missed messages without re-pairing the session.
// @Tags         Chat
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string  true  "Instance ID"
// @Success      200       {array}   whatsmiau.FindChatsRecord
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /chat/findChats/{instance} [post]
func (s *Chat) FindChats(ctx echo.Context) error {
	instanceID := ctx.Param("instance")
	if instanceID == "" {
		return utils.HTTPFail(ctx, http.StatusBadRequest, nil, "instance ID is required in the URL path")
	}

	response, err := s.whatsmiau.FindChats(ctx.Request().Context(), instanceID)
	if err != nil {
		zap.L().Error("Whatsmiau.FindChats failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to list chats")
	}

	return ctx.JSON(http.StatusOK, response)
}

// FindMessages godoc
// @Summary      List stored messages for a chat
// @Description  Evolution API v2 compatible endpoint. Paginated, most recent first — used by CartZap's backfill (sync:mensagens).
// @Tags         Chat
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                     true  "Instance ID"
// @Param        body      body      dto.FindMessagesRequest    true  "Filter and pagination"
// @Success      200       {object}  whatsmiau.FindMessagesResponse
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      422       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /chat/findMessages/{instance} [post]
func (s *Chat) FindMessages(ctx echo.Context) error {
	instanceID := ctx.Param("instance")
	if instanceID == "" {
		return utils.HTTPFail(ctx, http.StatusBadRequest, nil, "instance ID is required in the URL path")
	}

	var request dto.FindMessagesRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}

	page := request.Page
	if page <= 0 {
		page = 1
	}
	limit := request.Limit
	if limit <= 0 {
		limit = 50
	}

	response, err := s.whatsmiau.FindMessages(ctx.Request().Context(), instanceID, request.Where.Key.RemoteJid, page, limit)
	if err != nil {
		zap.L().Error("Whatsmiau.FindMessages failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to list messages")
	}

	return ctx.JSON(http.StatusOK, response)
}

// NumberExists godoc
// @Summary      Check if numbers exist on WhatsApp
// @Description  Checks whether the given phone numbers are registered on WhatsApp
// @Tags         Chat
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                    true  "Instance ID"
// @Param        body      body      dto.NumberExistsRequest    true  "Numbers to check"
// @Success      200       {array}   object                    "List of number existence results"
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      422       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /chat/whatsappNumbers/{instance} [post]
func (s *Chat) NumberExists(ctx echo.Context) error {
	instanceID := ctx.Param("instance")
	if instanceID == "" {
		return utils.HTTPFail(ctx, http.StatusBadRequest, nil, "instance ID is required in the URL path")
	}

	var request dto.NumberExistsRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}

	response, err := s.whatsmiau.NumberExists(ctx.Request().Context(), &whatsmiau.NumberExistsRequest{
		InstanceID: instanceID,
		Numbers:    request.Numbers,
	})
	if err != nil {
		zap.L().Error("Whatsmiau.NumberExists failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to check numbers")
	}

	return ctx.JSON(http.StatusOK, response)
}

// FetchProfilePictureUrl godoc
// @Summary      Fetch WhatsApp profile picture URL
// @Description  Returns the full-resolution profile picture URL for a given number. Evolution API compatible.
// @Tags         Chat
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                              true  "Instance ID"
// @Param        body      body      dto.FetchProfilePictureUrlRequest   true  "Number to look up"
// @Success      200       {object}  dto.FetchProfilePictureUrlResponse
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      404       {object}  utils.HTTPErrorResponse
// @Failure      422       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /chat/fetchProfilePictureUrl/{instance} [post]
func (s *Chat) FetchProfilePictureUrl(ctx echo.Context) error {
	instanceID := ctx.Param("instance")
	if instanceID == "" {
		return utils.HTTPFail(ctx, http.StatusBadRequest, nil, "instance ID is required in the URL path")
	}

	var request dto.FetchProfilePictureUrlRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}

	response, err := s.whatsmiau.FetchProfilePictureUrl(ctx.Request().Context(), &whatsmiau.FetchProfilePictureUrlRequest{
		InstanceID: instanceID,
		Number:     request.Number,
	})
	if err != nil {
		if errors.Is(err, whatsmeow.ErrClientIsNil) {
			return utils.HTTPFail(ctx, http.StatusNotFound, err, "instance not found or not connected")
		}
		if errors.Is(err, whatsmiau.ErrEmptyNumber) {
			return utils.HTTPFail(ctx, http.StatusBadRequest, err, "number is empty")
		}
		zap.L().Error("Whatsmiau.FetchProfilePictureUrl failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to fetch profile picture")
	}

	return ctx.JSON(http.StatusOK, dto.FetchProfilePictureUrlResponse{
		Wuid:              response.Wuid,
		ProfilePictureURL: response.ProfilePictureURL,
	})
}

// FetchProfile godoc
// @Summary      Fetch WhatsApp profile display name
// @Description  Returns the display name (full name, push name, business name) for a given number, read from the local contact store. Evolution API compatible.
// @Tags         Chat
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                      true  "Instance ID"
// @Param        body      body      dto.FetchProfileRequest      true  "Number to look up"
// @Success      200       {object}  dto.FetchProfileResponse
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      404       {object}  utils.HTTPErrorResponse
// @Failure      422       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /chat/fetchProfile/{instance} [post]
func (s *Chat) FetchProfile(ctx echo.Context) error {
	instanceID := ctx.Param("instance")
	if instanceID == "" {
		return utils.HTTPFail(ctx, http.StatusBadRequest, nil, "instance ID is required in the URL path")
	}

	var request dto.FetchProfileRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}

	response, err := s.whatsmiau.FetchProfile(ctx.Request().Context(), &whatsmiau.FetchProfileRequest{
		InstanceID: instanceID,
		Number:     request.Number,
	})
	if err != nil {
		if errors.Is(err, whatsmeow.ErrClientIsNil) {
			return utils.HTTPFail(ctx, http.StatusNotFound, err, "instance not found or not connected")
		}
		if errors.Is(err, whatsmiau.ErrEmptyNumber) {
			return utils.HTTPFail(ctx, http.StatusBadRequest, err, "number is empty")
		}
		zap.L().Error("Whatsmiau.FetchProfile failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to fetch profile")
	}

	return ctx.JSON(http.StatusOK, dto.FetchProfileResponse{
		Wuid:         response.Wuid,
		Name:         response.Name,
		PushName:     response.PushName,
		BusinessName: response.BusinessName,
		IsBusiness:   response.IsBusiness,
	})
}

// DeleteMessageForEveryone godoc
// @Summary      Revoke a sent message for everyone
// @Description  Deletes a previously sent message from WhatsApp ("apagar pra todos"). Only works on
// @Description  messages sent by the bot/instance and within ~2 days from when they were sent.
// @Tags         Chat
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                              true  "Instance ID"
// @Param        body      body      dto.DeleteMessageForEveryoneRequest  true  "Message key to revoke"
// @Success      200       {object}  map[string]interface{}              "Empty object on success"
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      404       {object}  utils.HTTPErrorResponse
// @Failure      422       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /instance/{instance}/chat/delete-message-for-everyone [post]
// @Router       /chat/deleteMessageForEveryone/{instance} [post]
func (s *Chat) DeleteMessageForEveryone(ctx echo.Context) error {
	var request dto.DeleteMessageForEveryoneRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}

	jid, err := numberToJid(request.Key.RemoteJid)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid remoteJid")
	}

	if err := s.whatsmiau.RevokeMessage(&whatsmiau.RevokeMessageRequest{
		InstanceID: request.InstanceID,
		RemoteJID:  jid,
		MessageID:  request.Key.ID,
	}); err != nil {
		zap.L().Error("Whatsmiau.RevokeMessage failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to revoke message")
	}

	return ctx.JSON(http.StatusOK, map[string]interface{}{})
}

// BlockContact godoc
// @Summary      Block or unblock a contact
// @Description  Blocks (or unblocks, with unblock=true) a contact on the connected WhatsApp
// @Description  account — same effect as doing it from the phone app. Once blocked, the
// @Description  contact can no longer send messages to this account.
// @Tags         Chat
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                    true  "Instance ID"
// @Param        body      body      dto.BlockContactRequest  true  "Number to block/unblock"
// @Success      200       {object}  map[string]interface{}   "Empty object on success"
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      404       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /instance/{instance}/chat/block-contact [post]
// @Router       /chat/blockContact/{instance} [post]
func (s *Chat) BlockContact(ctx echo.Context) error {
	var request dto.BlockContactRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}

	jid, err := numberToJid(request.Number)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid number")
	}

	var lidJID *types.JID
	if request.Lid != "" {
		parsed, err := types.ParseJID(request.Lid)
		if err != nil {
			return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid lid")
		}
		lidJID = &parsed
	}

	if err := s.whatsmiau.BlockContact(ctx.Request().Context(), &whatsmiau.BlockContactRequest{
		InstanceID: request.InstanceID,
		RemoteJID:  jid,
		Unblock:    request.Unblock,
		LID:        lidJID,
	}); err != nil {
		if errors.Is(err, whatsmeow.ErrClientIsNil) {
			return utils.HTTPFail(ctx, http.StatusNotFound, err, "instance not found or not connected")
		}
		zap.L().Error("Whatsmiau.BlockContact failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to block contact")
	}

	return ctx.JSON(http.StatusOK, map[string]interface{}{})
}

// EditMessage godoc
// @Summary      Edit a previously sent message
// @Description  Edits a message that was already sent via this instance. WhatsApp enforces a ~15min
// @Description  window server-side. Only text (Conversation) is supported in V1; caption editing
// @Description  for media will need a separate Message shape.
// @Tags         Chat
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                  true  "Instance ID"
// @Param        body      body      dto.EditMessageRequest  true  "Message key + new conversation text"
// @Success      200       {object}  map[string]interface{}  "Empty object on success"
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      422       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /instance/{instance}/chat/edit-message [post]
// @Router       /chat/editMessage/{instance} [post]
func (s *Chat) EditMessage(ctx echo.Context) error {
	var request dto.EditMessageRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}

	jid, err := numberToJid(request.Key.RemoteJid)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid remoteJid")
	}

	if err := s.whatsmiau.EditMessage(&whatsmiau.EditMessageRequest{
		InstanceID: request.InstanceID,
		RemoteJID:  jid,
		MessageID:  request.Key.ID,
		NewText:    request.Message.Conversation,
	}); err != nil {
		zap.L().Error("Whatsmiau.EditMessage failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to edit message")
	}

	return ctx.JSON(http.StatusOK, map[string]interface{}{})
}

// UpdatePrivacySettings godoc
// @Summary      Update WhatsApp privacy settings (read receipts, last seen, etc.)
// @Description  Changes one or more WhatsApp privacy settings on the linked account. Each field is
// @Description  optional — only fields with a non-empty value are applied. Fields are applied
// @Description  independently, so a single bad field does not block the others; per-field errors are
// @Description  reported in the response. Note: read receipts is bilateral — turning it off means
// @Description  you neither send nor see read receipts (2 blue ticks).
// @Tags         Chat
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                            true  "Instance ID"
// @Param        body      body      dto.UpdatePrivacySettingsRequest  true  "Privacy fields to update"
// @Success      200       {object}  dto.UpdatePrivacySettingsResponse
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      422       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /instance/{instance}/chat/privacy-settings [post]
// @Router       /chat/updatePrivacySettings/{instance} [post]
func (s *Chat) UpdatePrivacySettings(ctx echo.Context) error {
	var request dto.UpdatePrivacySettingsRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}

	result, err := s.whatsmiau.UpdatePrivacySettings(ctx.Request().Context(), &whatsmiau.UpdatePrivacySettingsRequest{
		InstanceID:   request.InstanceID,
		ReadReceipts: request.ReadReceipts,
		Profile:      request.Profile,
		GroupAdd:     request.GroupAdd,
		Last:         request.Last,
		Status:       request.Status,
		Online:       request.Online,
		CallAdd:      request.CallAdd,
	})
	if err != nil {
		if errors.Is(err, whatsmeow.ErrClientIsNil) {
			return utils.HTTPFail(ctx, http.StatusNotFound, err, "instance not found or not connected")
		}
		if errors.Is(err, whatsmeow.ErrNotLoggedIn) {
			return utils.HTTPFail(ctx, http.StatusBadRequest, err, "instance is not logged in")
		}
		zap.L().Error("Whatsmiau.UpdatePrivacySettings failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to update privacy settings")
	}

	return ctx.JSON(http.StatusOK, dto.UpdatePrivacySettingsResponse{
		Applied: result.Applied,
		Errors:  result.Errors,
	})
}
