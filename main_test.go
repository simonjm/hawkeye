package main

import "testing"

func TestShouldConvertAudio(t *testing.T) {
	filename := "/home/joe/Videos/girls.s03e08.720p.hdtv.x264.mkv"
	result, err := shouldConvertAudio(filename)
	if err != nil {
		t.Error(err)
	}

	if result != true {
		t.Errorf("Expected %s audo to be converted to aac", filename)
	}
}

func TestShouldConvertAudioWithAAC(t *testing.T) {
	filename := "/home/joe/Videos/The Simpsons [1.01] Simpsons Roasting On An Open Fire.mp4"
	result, err := shouldConvertAudio(filename)
	if err != nil {
		t.Error(err)
	}

	if result == true {
		t.Errorf("Expected %s audio to already be aac", filename)
	}
}
