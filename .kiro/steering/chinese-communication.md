# 中文交流和代码规范

## 语言要求

### 交流语言

- 所有与用户的对话必须使用中文
- 技术术语可以保留英文，但需要提供中文解释
- 错误信息和提示信息必须使用中文

### 代码注释规范

- 所有代码注释必须使用中文
- 函数和方法的文档注释必须使用中文
- 变量和常量的说明注释必须使用中文
- TODO、FIXME 等标记后的说明必须使用中文

### 文档规范

- README 文件和技术文档必须使用中文
- API 文档和接口说明必须使用中文
- 配置文件中的注释必须使用中文

## 代码示例

```go
// 用户信息结构体
type User struct {
    ID   int    `json:"id"`   // 用户唯一标识
    Name string `json:"name"` // 用户姓名
}

// GetId method  获取用户id
func (u *User) GetId(){
    // TODO
}

// GetUserByID function  根据用户ID获取用户信息
func GetUserByID(id int) (*User, error) {
    // TODO: 实现数据库查询逻辑
    return nil, nil
}
```

## 执行原则

- 在编写任何代码时，确保所有注释都使用中文
- 在解释代码逻辑时，使用中文进行说明
- 在创建文档时，优先使用中文
- 保持代码的可读性和专业性
