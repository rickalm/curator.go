package curator

import (
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/samuel/go-zookeeper/zk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type infof func(format string, args ...interface{})

type mockCloseable struct {
	mock.Mock

	crash bool
}

func (c *mockCloseable) Close() error {
	if c.crash {
		panic(errors.New("panic"))
	}

	return c.Called().Error(0)
}

type mockTracerDriver struct {
	mock.Mock
}

func (t *mockTracerDriver) AddTime(name string, d time.Duration) {
	t.Called(name, d)
}

func (t *mockTracerDriver) AddCount(name string, increment int) {
	t.Called(name, increment)
}

type mockRetrySleeper struct {
	mock.Mock
}

func (s *mockRetrySleeper) SleepFor(time time.Duration) error {
	return s.Called(time).Error(0)
}

type mockConn struct {
	mock.Mock

	operations []interface{}

	log infof
}

func (c *mockConn) AddAuth(scheme string, auth []byte) error {
	args := c.Called(scheme, auth)

	return args.Error(0)
}

func (c *mockConn) Close() {
	c.Called()
}

func (c *mockConn) Create(path string, data []byte, flags int32, acls []zk.ACL) (string, error) {
	/*
		if c.log != nil {
			c.log("Before Create(\"%s\", []byte(\"%s\"), %d, %v)", path, data, flags, acls)

			if len(path) == 0 {
				panic(path)
			}
		}
	*/
	args := c.Called(path, data, flags, acls)

	createPath := args.String(0)
	err := args.Error(1)

	if c.log != nil {
		c.log("Create(path=\"%s\", data=[]byte(\"%s\"), flags=%d, alcs=%v) (createdPath=\"%s\", error=%v)", path, data, flags, acls, createPath, err)
	}

	return createPath, err
}

func (c *mockConn) Exists(path string) (bool, *zk.Stat, error) {
	args := c.Called(path)

	exists := args.Bool(0)
	stat, _ := args.Get(1).(*zk.Stat)
	err := args.Error(2)

	if c.log != nil {
		c.log("Exists(path=\"%s\")(exists=%v, stat=%v, error=%v)", path, exists, stat, err)
	}

	return exists, stat, err
}

func (c *mockConn) ExistsW(path string) (bool, *zk.Stat, <-chan zk.Event, error) {
	args := c.Called(path)

	exists := args.Bool(0)
	stat, _ := args.Get(1).(*zk.Stat)
	events, _ := args.Get(2).(chan zk.Event)
	err := args.Error(3)

	if c.log != nil {
		c.log("ExistsW(path=\"%s\")(exists=%v, stat=%v, events=%v, error=%v)", path, exists, stat, events, err)
	}

	return exists, stat, events, err
}

func (c *mockConn) Delete(path string, version int32) error {
	args := c.Called(path, version)

	err := args.Error(0)

	if c.log != nil {
		c.log("Delete(path=\"%s\", version=%d) error=%v", path, version, err)
	}

	return err
}

func (c *mockConn) Get(path string) ([]byte, *zk.Stat, error) {
	args := c.Called(path)

	data, _ := args.Get(0).([]byte)
	stat, _ := args.Get(1).(*zk.Stat)

	return data, stat, args.Error(2)
}

func (c *mockConn) GetW(path string) ([]byte, *zk.Stat, <-chan zk.Event, error) {
	args := c.Called(path)

	data, _ := args.Get(0).([]byte)
	stat, _ := args.Get(1).(*zk.Stat)
	events, _ := args.Get(2).(chan zk.Event)

	return data, stat, events, args.Error(3)
}

func (c *mockConn) Set(path string, data []byte, version int32) (*zk.Stat, error) {
	args := c.Called(path, data, version)

	stat, _ := args.Get(0).(*zk.Stat)

	return stat, args.Error(1)
}

func (c *mockConn) Children(path string) ([]string, *zk.Stat, error) {
	args := c.Called(path)

	children, _ := args.Get(0).([]string)
	stat, _ := args.Get(1).(*zk.Stat)
	err := args.Error(2)

	if c.log != nil {
		c.log("Children(path=\"%s\")(children=%v, stat=%v, error=%v)", path, children, stat, err)
	}

	return children, stat, err
}

func (c *mockConn) ChildrenW(path string) ([]string, *zk.Stat, <-chan zk.Event, error) {
	args := c.Called(path)

	children, _ := args.Get(0).([]string)
	stat, _ := args.Get(1).(*zk.Stat)
	events, _ := args.Get(2).(chan zk.Event)
	err := args.Error(3)

	if c.log != nil {
		c.log("ChildrenW(path=\"%s\")(children=%v, stat=%v, events=%v, error=%v)", path, children, stat, events, err)
	}

	return children, stat, events, err
}

func (c *mockConn) GetACL(path string) ([]zk.ACL, *zk.Stat, error) {
	args := c.Called(path)

	acls, _ := args.Get(0).([]zk.ACL)
	stat, _ := args.Get(1).(*zk.Stat)

	return acls, stat, args.Error(2)
}

func (c *mockConn) SetACL(path string, acls []zk.ACL, version int32) (*zk.Stat, error) {
	args := c.Called(path, acls, version)

	stat, _ := args.Get(0).(*zk.Stat)

	return stat, args.Error(1)
}

func (c *mockConn) Multi(ops ...interface{}) ([]zk.MultiResponse, error) {
	c.operations = append(c.operations, ops...)

	args := c.Called(ops)

	res, _ := args.Get(0).([]zk.MultiResponse)
	err := args.Error(1)

	if c.log != nil {
		c.log("Multi(ops=%v)(responses=%v, error=%v)", ops, res, err)
	}

	return res, err
}

func (c *mockConn) Sync(path string) (string, error) {
	args := c.Called(path)

	return args.String(0), args.Error(1)
}

type mockZookeeperDialer struct {
	mock.Mock

	log infof
}

func (d *mockZookeeperDialer) Dial(connString string, sessionTimeout time.Duration, canBeReadOnly bool) (ZookeeperConnection, <-chan zk.Event, error) {
	args := d.Called(connString, sessionTimeout, canBeReadOnly)

	conn, _ := args.Get(0).(ZookeeperConnection)
	events, _ := args.Get(1).(chan zk.Event)
	err := args.Error(2)

	if d.log != nil {
		d.log("Dial(connectString=\"%s\", sessionTimeout=%v, canBeReadOnly=%v)(conn=%p, events=%v, error=%v)", connString, sessionTimeout, canBeReadOnly, conn, events, err)
	}

	return conn, events, err
}

type mockCompressionProvider struct {
	mock.Mock

	log infof
}

func (p *mockCompressionProvider) Compress(path string, data []byte) ([]byte, error) {
	args := p.Called(path, data)

	compressedData, _ := args.Get(0).([]byte)
	err := args.Error(1)

	if p.log != nil {
		p.log("Compress(path=\"%s\", data=[]byte(\"%s\"))(compressedData=[]byte(\"%s\"), error=%v)", path, data, compressedData, err)
	}

	return compressedData, err
}

func (p *mockCompressionProvider) Decompress(path string, compressedData []byte) ([]byte, error) {
	args := p.Called(path, compressedData)

	data, _ := args.Get(0).([]byte)
	err := args.Error(1)

	if p.log != nil {
		p.log("Decompress(path=\"%s\", compressedData=[]byte(\"%s\"))(data=[]byte(\"%s\"), error=%v)", path, compressedData, data, err)
	}

	return data, err
}

type mockACLProvider struct {
	mock.Mock
}

func (p *mockACLProvider) GetDefaultAcl() []zk.ACL {
	args := p.Called()

	return args.Get(0).([]zk.ACL)
}

func (p *mockACLProvider) GetAclForPath(path string) []zk.ACL {
	args := p.Called(path)

	return args.Get(0).([]zk.ACL)
}

type mockEnsurePath struct {
	mock.Mock

	log infof
}

func (e *mockEnsurePath) Ensure(client *CuratorZookeeperClient) error {
	args := e.Mock.Called(client)

	err := args.Error(0)

	if e.log != nil {
		e.log("Ensure(client=%p) error=%v", client, err)
	}

	return err
}

func (e *mockEnsurePath) ExcludingLast() EnsurePath {
	args := e.Mock.Called()

	ret, _ := args.Get(0).(EnsurePath)

	if e.log != nil {
		e.log("ExcludingLast() EnsurePath=%p", ret)
	}

	return ret
}

type mockEnsurePathHelper struct {
	mock.Mock

	log infof
}

func (h *mockEnsurePathHelper) Ensure(client *CuratorZookeeperClient, path string, makeLastNode bool) error {
	args := h.Called(client, path, makeLastNode)

	err := args.Error(0)

	if h.log != nil {
		h.log("Ensure(client=%p, path=\"%s\", makeLastNode=%v) error=%v", client, path, makeLastNode, err)
	}

	return err
}

type mockZookeeperClient struct {
	conn     *mockConn
	dialer   *mockZookeeperDialer
	compress *mockCompressionProvider
	builder  *CuratorFrameworkBuilder
	events   chan zk.Event
	wg       sync.WaitGroup
}

func newMockZookeeperClient() *mockZookeeperClient {
	c := &mockZookeeperClient{
		conn:     &mockConn{},
		dialer:   &mockZookeeperDialer{},
		compress: &mockCompressionProvider{},
		events:   make(chan zk.Event),
	}

	c.builder = &CuratorFrameworkBuilder{
		ZookeeperDialer:     c.dialer,
		EnsembleProvider:    &fixedEnsembleProvider{"connectString"},
		CompressionProvider: c.compress,
		RetryPolicy:         NewRetryOneTime(0),
		DefaultData:         []byte("default"),
	}

	return c
}

func (c *mockZookeeperClient) WithNamespace(namespace string) *mockZookeeperClient {
	c.builder.Namespace = namespace

	return c
}

func (c *mockZookeeperClient) Test(t *testing.T, callback interface{}) {
	c.conn.log = t.Logf
	c.dialer.log = t.Logf
	c.compress.log = t.Logf

	client := c.builder.Build()

	c.dialer.On("Dial", c.builder.EnsembleProvider.ConnectionString(), DEFAULT_CONNECTION_TIMEOUT, c.builder.CanBeReadOnly).Return(c.conn, c.events, nil).Once()

	assert.NoError(t, client.Start())

	fn := reflect.TypeOf(callback)

	assert.Equal(t, reflect.Func, fn.Kind())

	args := make([]reflect.Value, fn.NumIn())

	waiting := false

	for i := 0; i < fn.NumIn(); i++ {
		switch argType := fn.In(i); argType {
		case reflect.TypeOf(c.builder):
			args[i] = reflect.ValueOf(c.builder)

		case reflect.TypeOf((*CuratorFramework)(nil)).Elem():
			args[i] = reflect.ValueOf(client)

		case reflect.TypeOf((*ZookeeperConnection)(nil)).Elem(), reflect.TypeOf(c.conn):
			args[i] = reflect.ValueOf(c.conn)

		case reflect.TypeOf((*ZookeeperDialer)(nil)).Elem(), reflect.TypeOf(c.dialer):
			args[i] = reflect.ValueOf(c.dialer)

		case reflect.TypeOf((*ZookeeperDialer)(nil)).Elem(), reflect.TypeOf(c.compress):
			args[i] = reflect.ValueOf(c.compress)

		case reflect.TypeOf(c.events):
			args[i] = reflect.ValueOf(c.events)

		case reflect.TypeOf(&c.wg):
			args[i] = reflect.ValueOf(&c.wg)
			c.wg.Add(1)
			waiting = true

		default:
			t.Errorf("unknown arg type: %s", fn.In(i))
		}
	}

	reflect.ValueOf(callback).Call(args)

	if waiting {
		c.wg.Wait()
	}

	c.conn.On("Close").Return().Once()

	assert.NoError(t, client.Close())

	close(c.events)

	c.conn.AssertExpectations(t)
	c.dialer.AssertExpectations(t)
	c.compress.AssertExpectations(t)
}

type mockClientTestSuite struct {
	suite.Suite
}

func (s *mockClientTestSuite) WithClient(callback interface{}) {
	newMockZookeeperClient().Test(s.T(), callback)
}

func (s *mockClientTestSuite) WithClientAndNamespace(namespace string, callback interface{}) {
	newMockZookeeperClient().WithNamespace(namespace).Test(s.T(), callback)
}
