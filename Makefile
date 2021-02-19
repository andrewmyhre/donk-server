build:
	docker build --build-arg HOME_URL=https://donk-home-gk2td4ji5q-uc.a.run.app/ --build-arg API_URL=https://donk-server-gk2td4ji5q-uc.a.run.app/ -t gcr.io/donk-305000/donk-server .