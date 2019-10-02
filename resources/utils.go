package resources

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const (
	entityNameSeparator = "__"
	translatesComa      = ','
)

var (
	errInvalidFileNameFormat = "invalid file name. Expecting: '%sName__systemApplicationId'. Found: '%s'"
)

func ReadAllLines(
	csvReader *csv.Reader,
	batchSize int,
	onBatch func(batch [][]string, lastReadCount int, totalRead int) error,
	onError func(err error) bool,
) error {
	total := 0
	i := 0
	translates := make([][]string, batchSize)
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			if onError != nil && !onError(err) {
				return err
			} else {
				continue
			}
		}
		translates[i] = record
		i++
		total++
		if total%batchSize == 0 {
			if err := onBatch(translates, i, total); err != nil {
				if onError != nil && !onError(err) {
					return err
				} else {
					continue
				}
			}
			i = 0
			translates = make([][]string, batchSize)
		}
	}
	return onBatch(translates, i, total)
}

func SplitEntityName(fullName, entityType string) (string, int32, error) {
	parts := strings.Split(fullName, entityNameSeparator)
	if len(parts) != 2 {
		return "", 0, fmt.Errorf(errInvalidFileNameFormat, entityType, fullName)
	}
	name := parts[0]
	entityId := 0
	if id, err := strconv.Atoi(parts[1]); err != nil {
		return "", 0, fmt.Errorf(errInvalidFileNameFormat, entityType, fullName)
	} else {
		entityId = id
	}
	return name, int32(entityId), nil
}

func NewCsvReader(r io.Reader) *csv.Reader {
	csvReader := csv.NewReader(r)
	csvReader.Comma = translatesComa
	return csvReader
}

func NewCsvWriter(w io.Writer) *csv.Writer {
	csvWriter := csv.NewWriter(w)
	csvWriter.Comma = translatesComa
	return csvWriter
}
