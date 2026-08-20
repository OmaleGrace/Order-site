--
-- PostgreSQL database dump
--

\restrict dCmpCPhBsFfIqPY1DodIxOG6TTD95xUVx4cTVHejZirIICdWJdDckcxc3PACOfI

-- Dumped from database version 16.14 (Debian 16.14-1.pgdg13+1)
-- Dumped by pg_dump version 16.14 (Ubuntu 16.14-0ubuntu0.24.04.1)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Data for Name: menu_items; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.menu_items (id, name, description, price_kobo) FROM stdin;
1	Jollof Rice	Delicious Nigerian jollof rice	350000
2	Fried Rice	Fried rice with vegetables and chicken	400000
3	Spaghetti	Spaghetti with tomato sauce and chicken	300000
\.


--
-- Name: menu_items_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.menu_items_id_seq', 3, true);


--
-- PostgreSQL database dump complete
--

\unrestrict dCmpCPhBsFfIqPY1DodIxOG6TTD95xUVx4cTVHejZirIICdWJdDckcxc3PACOfI

