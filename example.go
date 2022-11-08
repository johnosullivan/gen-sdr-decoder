package main

import (
	"bufio"
	"errors"
	"fmt"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
)

// ABS Values
var (
	samplesPerMicrosec = 2
	thAmpDiff          = 0.8
	preamble           = []float64{1, 0, 1, 0, 0, 0, 0, 1, 0, 1, 0, 0, 0, 0, 0, 0}
	pBits              = 8
	fBits              = 112
)

func MinMax(array []float64) (float64, float64) {
	var max float64 = array[0]
	var min float64 = array[0]
	for _, value := range array {
		if max < value {
			max = value
		}
		if min > value {
			min = value
		}
	}
	return min, max
}

func raw_row(buf []float64) string {
	return fmt.Sprintf("Size: %d = %.18f %.18f %.18f...%.18f %.18f %.18f", len(buf), buf[0], buf[1], buf[2], buf[len(buf)-3], buf[len(buf)-2], buf[len(buf)-1])
}

func calc_noise(buf []float64) float64 {
	window := samplesPerMicrosec * 100
	totalLen := len(buf)
	row := totalLen / window
	index_diff := totalLen / window * window
	npa := buf[:index_diff]
	//log.Println("npa", len(npa))
	//log.Println("window", window)
	m := mat.NewDense(row, window, npa)
	//log.Println("b", m.IsEmpty())

	r, _ := m.Dims()
	means := make([]float64, r)
	for j := 0; j < r; j++ {
		//log.Println("R:", j, raw_row(m.RawRowView(j)))
		mean := stat.Mean(m.RawRowView(j), nil)
		//log.Println(mean)
		means[j] = mean
	}

	min, _ := MinMax(means)

	/*for i := 0; i < r; i++ {
		if (i + 1) == len(means) {
			continue
		}
		min = math.Min(means[i], means[i+1])
	}*/
	/*log.Println("")
	log.Println("M:", raw_row(means))
	log.Println("")*/

	//log.Println(MinMax(means))
	return min
}

func round(num float64) int {
	return int(num + math.Copysign(0.5, num))
}

func toFixed(num float64, precision int) float64 {
	output := math.Pow(10, float64(precision))
	return float64(round(num*output)) / output
}

func checkPreamble(pulses []float64) bool {
	if len(pulses) != 16 {
		return false
	}

	x := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 16}
	for i, _ := range x {
		if math.Abs(pulses[i]-preamble[i]) > thAmpDiff {
			return false
		}
	}

	return true
}

func parseBinToHex(s string) ([]string, error) {
	bStep := 4
	lBinary := len(s)
	//fCount := lBinary / bStep
	hex := []string{}
	for j := 0; j < lBinary; j += bStep {
		if j+bStep > lBinary {
			break
		}
		ui, err := strconv.ParseUint(s[j:j+bStep], 2, 64)
		if err != nil {
			continue
		}
		hex = append(hex, strings.ToUpper(strconv.FormatUint(ui, 16)))
	}
	return hex, nil
}

func parseHexToBin(s string) (string, error) {
	bin := ""
	for j := 0; j < len(s); j += 1 {
		i := s[j]
		n, err := strconv.ParseInt(string(i), 16, 32)
		if err != nil {
			return "", err
		}
		b := strconv.FormatInt(n, 2)
		if len(b) != 4 { // requires 4 characters
			pad := ""
			for a := 0; a < 4-len(b); a += 1 { // zero padding
				pad += "0"
			}
			b = pad + b
		}
		bin = bin + b
	}
	return bin, nil
}

func df(msg string) int {
	dfbin, err := parseHexToBin(msg[:2])
	if err != nil {
	}
	num, err := bin2int(dfbin[0:5])
	if err != nil {
	}
	min, _ := MinMax([]float64{float64(num), 24})
	return int(min)
}

func bin2int(bin string) (int, error) {
	if i, err := strconv.ParseInt(bin, 2, 64); err != nil {
		return 0, err
	} else {
		return int(i), nil
	}
}

func sum(array []int) int {
	result := 0
	for _, v := range array {
		result += v
	}
	return result
}

func crc(msg string) int {
	/*Mode-S Cyclic Redundancy Check.
	Detect if bit error occurs in the Mode-S message. When encode option is on, the checksum is generated.
		Args:
			msg: 28 bytes hexadecimal message string
			encode: True to encode the date only and return the checksum
			Returns:
			int: message checksum, or partity bits (encoder)
	*/
	// CRC generator
	G := []int{255, 250, 4, 128}
	msgBin, err := parseHexToBin(msg)
	if err != nil {
	}

	bStep := 8
	lBinary := len(msgBin)
	bInts := []int{}
	for j := 0; j < lBinary; j += bStep {
		if j+bStep > lBinary {
			break
		}
		i, err := bin2int(msgBin[j : j+bStep])
		if err != nil {
		}
		bInts = append(bInts, i)
	}
	//log.Println("bInts: ", bInts)
	for ibyte := 0; ibyte < len(bInts)-3; ibyte += 1 {
		for ibit := 0; ibit < 8; ibit += 1 {
			mask := 0x80 >> ibit
			bits := bInts[ibyte] & mask
			if bits > 0 {
				bInts[ibyte] = bInts[ibyte] ^ G[0]>>ibit
				bInts[ibyte+1] = bInts[ibyte+1] ^ (0xFF & ((G[0] << (8 - ibit)) | (G[1] >> ibit)))
				bInts[ibyte+2] = bInts[ibyte+2] ^ (0xFF & ((G[1] << (8 - ibit)) | (G[2] >> ibit)))
				bInts[ibyte+3] = bInts[ibyte+3] ^ (0xFF & ((G[2] << (8 - ibit)) | (G[3] >> ibit)))
			}
		}
	}
	//log.Println("bInts: ", bInts)
	result := (bInts[len(bInts)-3] << 16) | (bInts[len(bInts)-2] << 8) | bInts[len(bInts)-1]
	return result
}

func intContains(i []int, e int) bool {
	for _, a := range i {
		if a == e {
			return true
		}
	}
	return false
}

func checkMsg(msg string) bool {
	df := df(msg)
	ldf := len(msg)
	if df == 17 && ldf == 28 {
		if crc(msg) == 0 {
			return true
		}
	}
	if intContains([]int{20, 21}, df) && ldf == 28 {
		return true
	}
	if intContains([]int{4, 5, 11}, df) && ldf == 14 {
		return true
	}

	return false
}

func arrayToString(x []int, delim string) string {
	return strings.Trim(strings.Replace(fmt.Sprint(x), " ", delim, -1), "[]")
}

func main() {
	log.Printf("SIGNAL_BUFFER_DECODER_TEST")

	/*
			_calc_noise: 0.016428418086815284
			_noise_floor: 1000000.0
			window: 200
			total_len: 204800
			index_diff: 204800
			np_array_shape: (204800,)
			np_array: [0.93173368 0.79097174 0.89749962 ... 0.01663781 0.01240109 0.02286648]
			npr: [
		     [0.93173368 0.79097174 0.89749962 ... 0.54713964 0.48832584 1.18263743]
			 [0.44301577 0.62610149 0.35688429 ... 0.40621848 0.58621877 0.59775307]
			 [0.42458112 0.36657753 0.57840146 ... 0.29788438 0.63758966 0.07209716]
			 ...
			 [0.01999615 0.03373461 0.02286648 ... 0.01240109 0.01663781 0.02772968]
			 [0.01240109 0.00554594 0.01240109 ... 0.01663781 0.02986578 0.02772968]
			 [0.02772968 0.01240109 0.03720327 ... 0.01663781 0.01240109 0.02286648]
		    ]
			npm: [0.579455   0.57897812 0.57571838 ... 0.02125493 0.02099786 0.0216382 ]
			means: [0.579455   0.57897812 0.57571838 ... 0.02125493 0.02099786 0.0216382 ]

			noise_floor: 0.016428418086815284
			min_sig_amp: 0.051946657990509924
			8DA557FEE10A1A000000003BDB05 A557FE 0
			P_ADSB:  1667616041.938359 A557FE 28 8DA557FEE10A1A000000003BDB05

			[['8DA557FEE10A1A000000003BDB05', 1667616041.938359]]
	*/
	data := "8DA557FEE10A1A000000003BDB05.txt"

	_, err := os.Stat(data)
	if !errors.Is(err, os.ErrNotExist) {
		f, err := os.Open(data)

		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)

		var signalBuffer []float64
		for scanner.Scan() {
			if s, err := strconv.ParseFloat(scanner.Text(), 64); err == nil {
				signalBuffer = append(signalBuffer, s)
			}
		}
		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}

		log.Printf("SB_LEN: %d\n", len(signalBuffer))

		c := calc_noise(signalBuffer)
		noiseFloor := math.Min(c, 1e6)
		minSigAmp := 3.162 * noiseFloor

		//log.Println("NF: ", noiseFloor)
		//log.Println("MSA: ", minSigAmp)

		bufferLength := len(signalBuffer)

		i := 0
		for i < bufferLength {
			if signalBuffer[i] < minSigAmp {
				i += 1
				continue
			}

			if checkPreamble(signalBuffer[i : i+pBits*2]) {
				frameStart := i + pBits*2
				frameEnd := i + pBits*2 + (fBits+1)*2
				frameLength := (fBits + 1) * 2
				framePulses := signalBuffer[frameStart:frameEnd]

				_, max := MinMax(framePulses)
				threshold := max * 0.2

				var msgBin []int
				for j := 0; j < frameLength; j += 2 {
					p2 := framePulses[j : j+2]
					if len(p2) < 2 {
						break
					}
					if p2[0] < threshold && p2[1] < threshold {
						break
					} else {
						if p2[0] >= p2[1] {
							msgBin = append(msgBin, 1)
						} else {
							if p2[0] < p2[1] {
								msgBin = append(msgBin, 0)
							} else {
								break
							}
						}
					}
					i = frameStart + j
				}
				//log.Println("BinArray:", msgBin)
				if len(msgBin) > 0 {
					binData := arrayToString(msgBin, "")
					//log.Println("BData", binData)
					hexData, err := parseBinToHex(binData)
					if err == nil {
						//log.Println("HData", strings.Join(hexData, ""))
					}
					if checkMsg(strings.Join(hexData, "")) {
						log.Printf("%s - %s", strings.Join(hexData, ""), binData)
					}
				}
			} else {
				i += 1
			}
		}
	}
}
