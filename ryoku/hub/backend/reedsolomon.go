package main

// Reed-Solomon over GF(256) (polynomial 0x11d, generator 2), for the card code: a picture
// a chat app resized and recompressed can misread a few ticks, and nsym parity bytes
// repair up to nsym/2 bytes misread anywhere in the codeword.

import "errors"

var errTooManyErrors = errors.New("too many errors to correct")

var gfExp, gfLog = func() ([512]byte, [256]byte) {
	var exp [512]byte
	var log [256]byte
	x := 1
	for i := range 255 {
		exp[i] = byte(x)
		log[x] = byte(i)
		x <<= 1
		if x&0x100 != 0 {
			x ^= 0x11d
		}
	}
	for i := 255; i < 512; i++ {
		exp[i] = exp[i-255]
	}
	return exp, log
}()

func gfMul(a, b byte) byte {
	if a == 0 || b == 0 {
		return 0
	}
	return gfExp[int(gfLog[a])+int(gfLog[b])]
}

func gfPow(a byte, n int) byte {
	return gfExp[(int(gfLog[a])*n)%255]
}

func gfInverse(a byte) byte { return gfExp[255-int(gfLog[a])] }

// polynomials are highest degree first
func polyMul(p, q []byte) []byte {
	r := make([]byte, len(p)+len(q)-1)
	for j := range q {
		for i := range p {
			r[i+j] ^= gfMul(p[i], q[j])
		}
	}
	return r
}

func polyEval(p []byte, x byte) byte {
	y := p[0]
	for _, c := range p[1:] {
		y = gfMul(y, x) ^ c
	}
	return y
}

func polyScale(p []byte, x byte) []byte {
	r := make([]byte, len(p))
	for i, c := range p {
		r[i] = gfMul(c, x)
	}
	return r
}

func polyAdd(p, q []byte) []byte {
	n := max(len(p), len(q))
	r := make([]byte, n)
	for i := range p {
		r[i+n-len(p)] = p[i]
	}
	for i := range q {
		r[i+n-len(q)] ^= q[i]
	}
	return r
}

func rsGenerator(nsym int) []byte {
	g := []byte{1}
	for i := range nsym {
		g = polyMul(g, []byte{1, gfPow(2, i)})
	}
	return g
}

// rsEncode returns msg followed by nsym parity bytes
func rsEncode(msg []byte, nsym int) []byte {
	gen := rsGenerator(nsym)
	out := make([]byte, len(msg)+nsym)
	copy(out, msg)
	for i := range msg {
		c := out[i]
		if c != 0 {
			for j := 1; j < len(gen); j++ {
				out[i+j] ^= gfMul(gen[j], c)
			}
		}
	}
	copy(out, msg)
	return out
}

// rsDecode validates the bounded GF(256) codeword before correcting a private copy.
func rsDecode(code []byte, nsym int) ([]byte, error) {
	if nsym <= 0 || nsym >= len(code) || len(code) > 255 {
		return nil, errTooManyErrors
	}
	cw := append([]byte(nil), code...)
	synd := make([]byte, nsym)
	clean := true
	for i := range nsym {
		synd[i] = polyEval(cw, gfPow(2, i))
		if synd[i] != 0 {
			clean = false
		}
	}
	if clean {
		return cw[:len(cw)-nsym], nil
	}
	// Berlekamp-Massey: the error locator, lowest degree last
	errLoc, oldLoc := []byte{1}, []byte{1}
	for i := range nsym {
		delta := synd[i]
		for j := 1; j < len(errLoc) && j <= i; j++ {
			delta ^= gfMul(errLoc[len(errLoc)-1-j], synd[i-j])
		}
		oldLoc = append(oldLoc, 0)
		if delta != 0 {
			if len(oldLoc) > len(errLoc) {
				newLoc := polyScale(oldLoc, delta)
				oldLoc = polyScale(errLoc, gfInverse(delta))
				errLoc = newLoc
			}
			errLoc = polyAdd(errLoc, polyScale(oldLoc, delta))
		}
	}
	for len(errLoc) > 0 && errLoc[0] == 0 {
		errLoc = errLoc[1:]
	}
	nerr := len(errLoc) - 1
	if nerr <= 0 || nerr*2 > nsym {
		return nil, errTooManyErrors
	}
	// Chien search: roots of the locator give the error positions
	var pos []int
	n := len(cw)
	for i := range n {
		if polyEval(errLoc, gfPow(2, 255-i)) == 0 {
			pos = append(pos, n-1-i)
		}
	}
	if len(pos) != nerr {
		return nil, errTooManyErrors
	}
	// the magnitudes: with the positions known, syndrome i is sum Y_k * X_k^i, a small
	// linear system solved by elimination
	m := len(pos)
	a := make([][]byte, m)
	for i := range m {
		a[i] = make([]byte, m+1)
		for k, p := range pos {
			a[i][k] = gfPow(gfPow(2, n-1-p), i)
		}
		a[i][m] = synd[i]
	}
	for col := range m {
		piv := -1
		for r := col; r < m; r++ {
			if a[r][col] != 0 {
				piv = r
				break
			}
		}
		if piv < 0 {
			return nil, errTooManyErrors
		}
		a[col], a[piv] = a[piv], a[col]
		inv := gfInverse(a[col][col])
		for c := col; c <= m; c++ {
			a[col][c] = gfMul(a[col][c], inv)
		}
		for r := range m {
			if r != col && a[r][col] != 0 {
				f := a[r][col]
				for c := col; c <= m; c++ {
					a[r][c] ^= gfMul(f, a[col][c])
				}
			}
		}
	}
	for k, p := range pos {
		cw[p] ^= a[k][m]
	}
	for i := range nsym {
		if polyEval(cw, gfPow(2, i)) != 0 {
			return nil, errTooManyErrors
		}
	}
	return cw[:len(cw)-nsym], nil
}
