package crypto

import (
    "errors"
    "math/big"
)

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

var ErrInvalidBase58 = errors.New("invalid base58 string")

func Base58Encode(data []byte) string {
    if len(data) == 0 { return "" }
    value := new(big.Int).SetBytes(data)
    base := big.NewInt(58)
    zero := big.NewInt(0)
    var out []byte
    for value.Cmp(zero) > 0 {
        mod := new(big.Int)
        value.QuoRem(value, base, mod)
        out = append(out, base58Alphabet[mod.Int64()])
    }
    for _, b := range data {
        if b != 0 { break }
        out = append(out, base58Alphabet[0])
    }
    for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 { out[i], out[j] = out[j], out[i] }
    return string(out)
}

func Base58Decode(value string) ([]byte, error) {
    if value == "" { return []byte{}, nil }
    result := new(big.Int)
    base := big.NewInt(58)
    for _, ch := range value {
        idx := int64(-1)
        for i := 0; i < len(base58Alphabet); i++ { if rune(base58Alphabet[i]) == ch { idx = int64(i); break } }
        if idx < 0 { return nil, ErrInvalidBase58 }
        result.Mul(result, base)
        result.Add(result, big.NewInt(idx))
    }
    decoded := result.Bytes()
    zeros := 0
    for _, ch := range value { if ch == '1' { zeros++ } else { break } }
    if zeros > 0 { decoded = append(make([]byte, zeros), decoded...) }
    return decoded, nil
}