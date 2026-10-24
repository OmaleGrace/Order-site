--
-- PostgreSQL database dump
--

\restrict PCj5xoC522CYhiCY2EBscQdi0YmtBcZFKIgF94oVznasQwpFt9dzAzhNZihDaHR

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

COPY public.menu_items (id, name, description, price_kobo, image_url) FROM stdin;
1	Jollof Rice	Delicious Nigerian jollof rice	350000	https://images.unsplash.com/photo-1665332195309-9d75071138f0?q=80&w=870&auto=format&fit=crop&ixlib=rb-4.1.0&ixid=M3wxMjA3fDB8MHxwaG90by1wYWdlfHx8fGVufDB8fHx8fA%3D%3D
2	Fried Rice	Fried rice with vegetables and chicken	400000	https://images.unsplash.com/photo-1603133872878-684f208fb84b?q=80&w=1025&auto=format&fit=crop&ixlib=rb-4.1.0&ixid=M3wxMjA3fDB8MHxwaG90by1wYWdlfHx8fGVufDB8fHx8fA%3D%3D
3	Spaghetti	Spaghetti with tomato sauce and chicken	300000	https://images.unsplash.com/photo-1616299806579-c2e2c4ee8e57?q=80&w=1043&auto=format&fit=crop&ixlib=rb-4.1.0&ixid=M3wxMjA3fDB8MHxwaG90by1wYWdlfHx8fGVufDB8fHx8fA%3D%3D
4	Ofada Rice with Ayamase Sauce	Local ofada rice served with spicy green pepper sauce	280000	https://encrypted-tbn0.gstatic.com/images?q=tbn:ANd9GcTiBkhFvqxG-FAvVGSX2cq0aAwMO6MZxDTV-5R8KhBnDw&s=10
5	White Rice and Stew	Steamed white rice with rich tomato pepper stew	200000	https://encrypted-tbn0.gstatic.com/images?q=tbn:ANd9GcQadU_LlPWcHtEfbmILNXdQ-HvG_szSH8J3H5W9_a6iqw&s=10
6	Pounded Yam and Egusi Soup	Smooth pounded yam served with hearty egusi soup	250000	https://encrypted-tbn0.gstatic.com/images?q=tbn:ANd9GcTkjsHJxFjiTAhXtv7otfoT9Lwru0xaT4mcn_ebTUyAkg&s=10
7	Eba and Okra Soup	Garri swallow served with fresh okra soup	200000	https://encrypted-tbn0.gstatic.com/images?q=tbn:ANd9GcQGhMKPgSG5338v2Z1xSJeGmNbLHWtcnmakWxX-GGCWOQ&s=10
8	Amala and Ewedu	Yam flour swallow served with ewedu and gbegiri	220000	https://www.thepointersnewsonline.com/wp-content/uploads/2025/06/Amala-And-Ewedu.jpeg
9	Grilled Chicken	Chicken marinated and grilled to perfection	300000	https://plus.unsplash.com/premium_photo-1695931844305-b5dd90ab6138?q=80&w=699&auto=format&fit=crop&ixlib=rb-4.1.0&ixid=M3wxMjA3fDB8MHxwaG90by1wYWdlfHx8fGVufDB8fHx8fA%3D%3D
10	Peppered Beef (Asun)	Spicy grilled goat meat cut into small chunks	350000	https://assets.ekotoken.ng/wp-content/uploads/sites/5/2022/11/04094423/asun-600x600-1.jpg
11	Fried Fish	Whole fish deep fried and seasoned	280000	https://amunafoods.com/wp-content/uploads/2021/10/IMG_2459.jpg
12	Moin Moin	Steamed bean pudding with egg and fish	100000	https://www.yummymedley.com/wp-content/uploads/2017/07/Moi-Moi-moin-moin-1-500x375.jpg
13	Puff Puff	Sweet fried dough balls	70000	https://images.unsplash.com/photo-1664993085274-80c6ba725ccc?q=80&w=765&auto=format&fit=crop&ixlib=rb-4.1.0&ixid=M3wxMjA3fDB8MHxwaG90by1wYWdlfHx8fGVufDB8fHx8fA%3D%3D
14	Efo Riro	Rich vegetable soup with assorted meat and fish	250000	https://encrypted-tbn0.gstatic.com/images?q=tbn:ANd9GcRc1RK1nVXaENpOw5VFlgOhsW8IAa284r9UV_hR4l9eUg&s=10
16	Banga Soup	Palm nut soup with assorted meat	280000	https://proveg.org/ng/wp-content/uploads/sites/4/2024/04/Banga_soup-removebg-preview.png
17	Chapman	Nigeria's classic non-alcoholic cocktail	120000	https://www.dashofjazz.com/wp-content/uploads/2024/10/Dash-of-Jazz-Nigerian-Chapman-Drink-9.jpg
18	Zobo	Chilled hibiscus drink with natural spices	80000	https://proveg.org/ng/wp-content/uploads/sites/4/2024/04/Zobo-drink-scaled-e1696244205636-500x375.jpg
15	Pepper Soup (Goat Meat)	Spicy light soup with tender goat meat	300000	https://plus.unsplash.com/premium_photo-1723708871094-2c02cf5f5394?q=80&w=1064&auto=format&fit=crop&ixlib=rb-4.1.0&ixid=M3wxMjA3fDB8MHxwaG90by1wYWdlfHx8fGVufDB8fHx8fA%3D%3D
19	Fresh Bottled Water	75cl bottled water	30000	https://images.unsplash.com/photo-1561041695-d2fadf9f318c?w=500&auto=format&fit=crop&q=60&ixlib=rb-4.1.0&ixid=M3wxMjA3fDB8MHxzZWFyY2h8Mnx8Ym90dGxlJTIwd2F0ZXJ8ZW58MHx8MHx8fDA%3D
\.


--
-- Name: menu_items_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.menu_items_id_seq', 19, true);


--
-- PostgreSQL database dump complete
--

\unrestrict PCj5xoC522CYhiCY2EBscQdi0YmtBcZFKIgF94oVznasQwpFt9dzAzhNZihDaHR

