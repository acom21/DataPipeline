package main

import (
	"container/ring"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

// Интервал очистки кольцевого буфера
const bufferDrainInterval time.Duration = 5 * time.Second

// Размер кольцевого буфера
const bufferSize int = 10

// Написать источник данных для конвейера.источник данных -консоль.
func RingNewInt(size int) *ring.Ring {
	r := ring.New(size + 1)
	return r
}

// чтение из консоли и запись в канал поученных значений
func read(inputData chan<- int, exit chan bool) { //(<-chan int, <-chan bool) { //<-chan bool

	go func() {

		for {
			var u string
			_, err := fmt.Scanln(&u)
			if strings.EqualFold(u, "") {
				continue
			}
			if err != nil {
				panic(err)

			}
			if strings.EqualFold(u, "exit") {
				fmt.Println("goodbye")
				close(exit)
				return
			}
			i, err := strconv.Atoi(u)

			inputData <- i
		}
	}()
}

// Также написать код потребителя данных конвейера.
// Данные от конвейера можно направить снова в консоль построчно,
// сопроводив их каким-нибудь поясняющим текстом, например: «Получены данные …».

// Стадия фильтрации отрицательных чисел (не пропускать отрицательные числа).

// StageInt - Стадия конвейера, обрабатывающая целые числа
type StageInt func(<-chan bool, <-chan int) <-chan int

type PipeLineInt struct {
	stages []StageInt
	done   <-chan bool
}

// NewPipelineInt - Создание пайплайна обработки целых чисел
func NewPipelineInt(done <-chan bool, stages ...StageInt) *PipeLineInt {
	return &PipeLineInt{done: done, stages: stages}
}

//Отправка данных в пайплайн
func (p *PipeLineInt) AddDataInPipe(source <-chan int) <-chan int {
	var dataFromSource <-chan int = source
	for i := range p.stages {
		dataFromSource = func(stage StageInt, sourceChan <-chan int) <-chan int {
			return stage(p.done, sourceChan)
		}(p.stages[i], dataFromSource)
	}
	log.Println("added data to pipeline")

	return dataFromSource
}

func main() {

	inputData, done := make(chan int), make(chan bool)
	read(inputData, done)
	// Стадия фильтрации чисел, не кратных 3 (не пропускать такие числа), исключая и 0.
	negativeFilterStageInt := func(done <-chan bool, c <-chan int) <-chan int {
		convertedIntChan := make(chan int)
		go func() {
			for {
				select {
				case data := <-c:
					if data > 0 {
						convertedIntChan <- data
						log.Println("negative filter checked", data)
						continue
					}
					log.Println("negative filter not passed", data)
				case <-done:
					log.Println("negative numbers stage done")
					return
				}
			}
		}()
		return convertedIntChan
	}
	// стадия, фильтрующая числа, не кратные 3
	specialFilterStageInt := func(done <-chan bool, c <-chan int) <-chan int {
		filteredIntChan := make(chan int)
		go func() {
			for {
				select {
				case data := <-c:
					if data != 0 && data%3 == 0 {
						select {
						case filteredIntChan <- data:
							log.Println("multiple of 3 checked", data)
						case <-done:
							return
						}
						continue
					}
					log.Println("filter multiple of 3 not passed", data)
				case <-done:

					return
				}
			}
		}()
		return filteredIntChan
	}
	// Стадия буферизации данных в кольцевом буфере с интерфейсом,
	// применение кольцевого буфера из пакетв container/ring
	bufferStageInt := func(done <-chan bool, c <-chan int) <-chan int {
		bufferedIntChan := make(chan int)
		buffer := RingNewInt(bufferSize)
		go func() {
			for {
				select {
				case data := <-c:

					buffer.Value = data
					buffer = buffer.Next()
				case <-done:
					return
				}
			}
		}()
		// опустошение буфера
		go func() {
			for {
				select {
				case <-time.After(bufferDrainInterval):
					for i := 0; i < buffer.Len(); i++ {

						fmt.Println(i, buffer.Value)
						buffer = buffer.Next()
					}
				case <-done:
					return
				}
			}
		}()

		return bufferedIntChan
	}
	// Потребитель данных от пайплайна
	consumer := func(done <-chan bool, c <-chan int) {
		for {
			select {
			case data := <-c:
				fmt.Printf("Обработаны данные: %d\n", data)
			case <-done:
				return
			}
		}
	}

	pipeline := NewPipelineInt(done, negativeFilterStageInt, specialFilterStageInt, bufferStageInt)
	consumer(done, pipeline.AddDataInPipe(inputData))
}
