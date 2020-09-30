package structure

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

type MetricAddress struct {
	AddressConfiguration
	Path string `json:"path" schema:"Путь,путь, по которому доступны метрики"`
}

type MetricConfiguration struct {
	Address                MetricAddress `json:"address" schema:"Адрес HTTP сервера для публикации метрик"`
	Gc                     bool          `json:"gc" schema:"Статистика по работе сборщика мусора,включение/отключение сбора статистики"`
	CollectingGCPeriod     int32         `json:"collectingGCPeriod" schema:"Интервал сбора статистики по работе сборщика мусор,значение в секундах, через которое происходит повторный сбор статистики, по умолчанию: 10"`
	Memory                 bool          `json:"memory" schema:"Статиста по памяти,включение/отключение сбора статистики"`
	CollectingMemoryPeriod int32         `json:"collectingMemoryPeriod" schema:"Интервал сбора статистики по памяти,значение в секундах, через которое происходит повторный сбор статистики, по умолчанию: 10"`
}

type AddressConfiguration struct {
	Port string `json:"port" schema:"Порт"`
	IP   string `json:"ip" schema:"Хост"`
}

func (addressConfiguration *AddressConfiguration) GetAddress() string {
	return net.JoinHostPort(addressConfiguration.IP, addressConfiguration.Port)
}

type RedisConfiguration struct {
	Address   AddressConfiguration `schema:"Адрес Redis"`
	Username  string               `schema:"Логин"`
	Password  string               `schema:"Пароль"`
	DefaultDB int                  `schema:"База данных по умолчанию"`
	Sentinel  *RedisSentinel       `schema:"Настройки Sentinel"`
}

type RedisSentinel struct {
	MasterName        string   `schema:"Наименование мастера"`
	SentinelAddresses []string `schema:"Список адресов,host:port"`
	// Deprecated: для sentinel не нужен отдельный username
	SentinelUsername string `schema:"Логин Sentinel,deprecated: поле больше не используется"`
	SentinelPassword string `schema:"Пароль Sentinel"`
}

type RabbitConfig struct {
	Address  AddressConfiguration `valid:"required~Required" schema:"Адрес RabbitMQ"`
	Vhost    string               `schema:"Виртуальный хост,для изоляции очередей"`
	User     string               `schema:"Логин"`
	Password string               `schema:"Пароль"`
}

func (rc RabbitConfig) GetUri() string {
	if rc.User == "" {
		return fmt.Sprintf("amqp://%s/%s", rc.Address.GetAddress(), rc.Vhost)
	} else {
		return fmt.Sprintf("amqp://%s:%s@%s/%s", rc.User, rc.Password, rc.Address.GetAddress(), rc.Vhost)
	}
}

func (rc RabbitConfig) ReconnectionTimeout() time.Duration {
	/*timeout := rc.ReconnectionTimeoutMs
	if timeout <= 0 {
		timeout = defaultReconnectionTimeout
	}*/
	return 3 * time.Millisecond
}

type DBConfiguration struct {
	Address      string `valid:"required~Required" schema:"Адрес"`
	Schema       string `valid:"required~Required" schema:"Схема"`
	Database     string `valid:"required~Required" schema:"Название базы данных"`
	Port         string `valid:"required~Required" schema:"Порт"`
	Username     string `schema:"Логин"`
	Password     string `schema:"Пароль"`
	PoolSize     int    `schema:"Количество соединений в пуле,по умолчанию 10 соединений на каждое ядро"`
	CreateSchema bool   `schema:"Создание схемы,если включено, создает схему, если ее не существует"`
}

type NatsConfig struct {
	ClusterId       string               `valid:"required~Required" schema:"Идентификатор кластера"`
	Address         AddressConfiguration `valid:"required~Required" schema:"Адрес Nats"`
	PingAttempts    int                  `schema:"Максимальное количество попыток соединения,когда будет достигнут максимальное значение количества попыток соединение будет закрыто"`
	PintIntervalSec int                  `schema:"Интервал проверки соединения,значение в секундах, через которое происходит проверка соединения"`
	ClientId        string               `json:"-"`
}

type SocketConfiguration struct {
	Host                  string            `schema:"Хост"`
	Port                  string            `schema:"Порт"`
	Path                  string            `schema:"Путь"`
	Secure                bool              `schema:"Защищенное соединение,если включено используется https"`
	UrlParams             map[string]string `schema:"Параметры,"`
	ConnectionString      string            `schema:"Строка соединения"`
	ConnectionReadLimitKB int64             `schema:"Максимальное количество килобайт на чтение,при превышении соединение закрывается с ошибкой"`
}

type ElasticConfiguration struct {
	URL         string `schema:"Адрес"`
	Username    string `schema:"Логин"`
	Password    string `schema:"Пароль"`
	Sniff       *bool  `schema:"Механизм поиска нод в кластере,если включено, клиент подключается ко всем нодам в кластере"`
	Healthcheck *bool  `schema:"Проверка работоспособности нод,если включено, пингует ноды"`
}

func (ec *ElasticConfiguration) ConvertTo(elasticConfigPtr interface{}) error {
	if bytes, err := json.Marshal(ec); err != nil {
		return err
	} else if err := json.Unmarshal(bytes, elasticConfigPtr); err != nil {
		return err
	}
	return nil
}
