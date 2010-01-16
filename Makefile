


all: clean
	cd lib; make install
	cd main; make install


clean:
	cd lib; make clean
	cd main; make clean
