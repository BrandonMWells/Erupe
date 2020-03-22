package channelserver

import (
	"database/sql"
	"encoding/binary"
	"github.com/Andoryuuta/Erupe/server/channelserver/compression/nullcomp"
	"go.uber.org/zap"
)

const CharacterSaveRPPointer = 0x22D16

type CharacterSaveData struct {
	CharID uint32
	RP     uint16

	// Use provided setter/getter
	baseSaveData []byte
}

func GetCharacterSaveData(s *Session, charID uint32) (*CharacterSaveData, error) {
	result, err := s.server.db.Queryx(
		"SELECT id, savedata FROM characters WHERE id = $1",
		charID,
	)

	if err != nil {
		s.logger.Error(
			"failed to retrieve save data for character",
			zap.Error(err),
			zap.Uint32("charID", charID),
		)
		return nil, err
	}

	saveData := &CharacterSaveData{}
	var compressedBaseSave []byte

	if !result.Next() {
		s.logger.Error(
			"no results found for character save data",
			zap.Uint32("charID", charID),
		)
		return nil, err
	}

	err = result.Scan(&saveData.CharID, &compressedBaseSave)

	if err != nil {
		s.logger.Error(
			"failed to retrieve save data for character",
			zap.Error(err),
			zap.Uint32("charID", charID),
		)

		return nil, err
	}

	decompressedBaseSave, err := nullcomp.Decompress(compressedBaseSave)

	if err != nil {
		s.logger.Error("Failed to decompress savedata from db", zap.Error(err))
		return nil, err
	}

	saveData.SetBaseSaveData(decompressedBaseSave)

	return saveData, nil
}

func (save *CharacterSaveData) Save(s *Session, transaction *sql.Tx) error {
	// We need to update the save data byte array before we save it back to the DB
	save.updateSaveDataWithStruct()

	compressedData, err := nullcomp.Compress(save.baseSaveData)

	if err != nil {
		s.logger.Error("failed to compress saveData", zap.Error(err), zap.Uint32("charID", save.CharID))
		return err
	}

	updateSQL := `
		UPDATE characters 
			SET savedata=$1
		WHERE id=$2
	`

	if transaction != nil {
		_, err = transaction.Exec(updateSQL, compressedData, save.CharID)
	} else {
		_, err = s.server.db.Exec(updateSQL, compressedData, save.CharID)
	}

	if err != nil {
		s.logger.Error("failed to save character data", zap.Error(err), zap.Uint32("charID", save.CharID))
		return err
	}

	return nil
}

func (save *CharacterSaveData) BaseSaveData() []byte {
	return save.baseSaveData
}

func (save *CharacterSaveData) SetBaseSaveData(data []byte) {
	save.baseSaveData = data
	// After setting the new save byte array, we can extract the values to update our struct
	// This will be useful when we save it back, we use the struct values to overwrite the saveData
	save.updateStructWithSaveData()
}

// This will update the save struct with the values stored in the raw savedata arrays
func (save *CharacterSaveData) updateSaveDataWithStruct() {
	rpBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(rpBytes, save.RP)
	copy(save.baseSaveData[CharacterSaveRPPointer:CharacterSaveRPPointer+2], rpBytes)
}

// This will update the character save struct with the values stored in the raw savedata arrays
func (save *CharacterSaveData) updateStructWithSaveData() {
	save.RP = binary.LittleEndian.Uint16(save.baseSaveData[CharacterSaveRPPointer : CharacterSaveRPPointer+2])
}
