// Package crypto 封装 bcrypt 密码哈希与 AES-GCM 密文（00 §6）。
package crypto

import "golang.org/x/crypto/bcrypt"

// PasswordHasher 用 bcrypt 做密码哈希与校验（明文绝不落库，compact 不变量 6）。
type PasswordHasher struct {
	cost int
}

// NewPasswordHasher 构造哈希器；cost 越界则回落到 bcrypt 默认 cost。
func NewPasswordHasher(cost int) PasswordHasher {
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		cost = bcrypt.DefaultCost
	}
	return PasswordHasher{cost: cost}
}

// Hash 生成 bcrypt 哈希。
func (h PasswordHasher) Hash(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), h.cost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Compare 校验明文是否匹配 bcrypt 哈希；匹配返回 nil。
func (h PasswordHasher) Compare(hash, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}
