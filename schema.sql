--
-- PostgreSQL database dump
--

SET statement_timeout = 0;
SET lock_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SET check_function_bodies = false;
SET client_min_messages = warning;

--
-- Name: postgres; Type: COMMENT; Schema: -; Owner: postgres
--

COMMENT ON DATABASE postgres IS 'default administrative connection database';


--
-- Name: plpgsql; Type: EXTENSION; Schema: -; Owner: 
--

CREATE EXTENSION IF NOT EXISTS plpgsql WITH SCHEMA pg_catalog;


--
-- Name: EXTENSION plpgsql; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION plpgsql IS 'PL/pgSQL procedural language';


--
-- Name: pg_bigm; Type: EXTENSION; Schema: -; Owner: 
--

CREATE EXTENSION IF NOT EXISTS pg_bigm WITH SCHEMA public;


--
-- Name: EXTENSION pg_bigm; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION pg_bigm IS 'text similarity measurement and index searching based on bigrams';


--
-- Name: zhparser; Type: EXTENSION; Schema: -; Owner: 
--

CREATE EXTENSION IF NOT EXISTS zhparser WITH SCHEMA public;


--
-- Name: EXTENSION zhparser; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION zhparser IS 'a parser for full-text search of Chinese';


SET search_path = public, pg_catalog;

--
-- Name: filename_changed(); Type: FUNCTION; Schema: public; Owner: postgres
--

CREATE FUNCTION filename_changed() RETURNS trigger
    LANGUAGE plpgsql
    AS $$

begin

if not ascii(NEW.filename) = 42 then
insert into image_keywords select unnest(regexp_split_to_array(lower(NEW.filename), E'\\s+'));
end if;

return NEW;

end

$$;


ALTER FUNCTION public.filename_changed() OWNER TO postgres;

--
-- Name: new_article(text, integer, text, text, text, bigint, bigint, integer, integer, integer); Type: FUNCTION; Schema: public; Owner: postgres
--

CREATE FUNCTION new_article(ptitle text, ptag integer, pcontent text, praw text, ppreview text, pcreated_at bigint, pmodified_at bigint, pauthor integer, pparent integer, pcooldown integer) RETURNS bigint
    LANGUAGE plpgsql
    AS $$
DECLARE
    sec bigint;
BEGIN
    sec := COALESCE(EXTRACT(EPOCH FROM now())::bigint * 1000 - (
            select 
            "modified_at"
            from articles 
            where "author" = Pauthor
            order by "modified_at" desc
            limit 1
        ), 65536000);

    if sec >= Pcooldown * 1000 or Pcooldown = 0 then

        insert into articles ("title", "tag", "content", "raw", "preview", "created_at", "modified_at", "author", "original_author", "parent", "vector") 
                    values   (Ptitle , Ptag , Pcontent , Praw,  Ppreview , Pcreated_at , Pmodified_at , Pauthor , Pauthor, Pparent, to_tsvector('testzhcfg', Ptitle || ' ' || Praw));
        update articles set "modified_at" = Pmodified_at, "children" = "children" + 1 where "id" = Pparent;
        return 0;
    else
        return sec;
    end if;
END;
$$;


ALTER FUNCTION public.new_article(ptitle text, ptag integer, pcontent text, praw text, ppreview text, pcreated_at bigint, pmodified_at bigint, pauthor integer, pparent integer, pcooldown integer) OWNER TO postgres;

--
-- Name: new_user_registered(); Type: FUNCTION; Schema: public; Owner: postgres
--

CREATE FUNCTION new_user_registered() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
	begin
		insert into user_info ("id", "username") values (NEW.ID, NEW.username);
		return NEW;
	end
$$;


ALTER FUNCTION public.new_user_registered() OWNER TO postgres;

--
-- Name: update_article(integer, text, integer, integer, text, text, text, bigint, text, integer, text, text, bigint, integer); Type: FUNCTION; Schema: public; Owner: postgres
--

CREATE FUNCTION update_article(pid integer, ptitle text, ptag integer, pauthor integer, pcontent text, praw text, ppreview text, pmodified_at bigint, pold_title text, pold_author integer, pold_content text, pold_raw text, pold_modified_at bigint, pcooldown integer) RETURNS bigint
    LANGUAGE plpgsql
    AS $$
DECLARE
    sec bigint;
BEGIN
    sec := EXTRACT(EPOCH FROM now())::bigint * 1000 - (
            select 
            "modified_at" 
            from articles 
            where "author" = Pauthor
            order by "modified_at" desc
            limit 1
        );

    if sec >= Pcooldown * 1000 or Pcooldown = 0 then
        update articles set ("title", "tag", "author", "content", "raw", "preview", "modified_at", "rev", "vector") 
                            = (Ptitle, Ptag, Pauthor, Pcontent, Praw, Ppreview, Pmodified_at, "rev" + 1, to_tsvector('testzhcfg', Ptitle || ' ' || Praw)) 
                        where "id" = Pid;
        insert into history ("article_id", "date", "title", "content", "raw", "user_id") 
                        values (Pid, Pold_modified_at, Pold_title, Pold_content, Pold_raw, Pold_author);
        return 0;
    else
        return sec;
    end if;
END;
$$;


ALTER FUNCTION public.update_article(pid integer, ptitle text, ptag integer, pauthor integer, pcontent text, praw text, ppreview text, pmodified_at bigint, pold_title text, pold_author integer, pold_content text, pold_raw text, pold_modified_at bigint, pcooldown integer) OWNER TO postgres;

--
-- Name: testzhcfg; Type: TEXT SEARCH CONFIGURATION; Schema: public; Owner: postgres
--

CREATE TEXT SEARCH CONFIGURATION testzhcfg (
    PARSER = zhparser );

ALTER TEXT SEARCH CONFIGURATION testzhcfg
    ADD MAPPING FOR a WITH simple;

ALTER TEXT SEARCH CONFIGURATION testzhcfg
    ADD MAPPING FOR e WITH simple;

ALTER TEXT SEARCH CONFIGURATION testzhcfg
    ADD MAPPING FOR i WITH simple;

ALTER TEXT SEARCH CONFIGURATION testzhcfg
    ADD MAPPING FOR j WITH simple;

ALTER TEXT SEARCH CONFIGURATION testzhcfg
    ADD MAPPING FOR l WITH simple;

ALTER TEXT SEARCH CONFIGURATION testzhcfg
    ADD MAPPING FOR n WITH simple;

ALTER TEXT SEARCH CONFIGURATION testzhcfg
    ADD MAPPING FOR v WITH simple;


ALTER TEXT SEARCH CONFIGURATION testzhcfg OWNER TO postgres;

--
-- Name: article_id_seq; Type: SEQUENCE; Schema: public; Owner: coyove
--

CREATE SEQUENCE article_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE article_id_seq OWNER TO coyove;

SET default_tablespace = '';

SET default_with_oids = false;

--
-- Name: articles; Type: TABLE; Schema: public; Owner: coyove; Tablespace: 
--

CREATE TABLE articles (
    id integer DEFAULT nextval('article_id_seq'::regclass) NOT NULL,
    title text,
    tag integer,
    content text,
    created_at bigint NOT NULL,
    author integer,
    modified_at bigint NOT NULL,
    deleted boolean DEFAULT false,
    hits integer DEFAULT 0,
    locked boolean DEFAULT false,
    parent integer DEFAULT 0,
    children integer DEFAULT 0,
    preview text DEFAULT ''::text,
    rev integer DEFAULT 0,
    original_author integer,
    read boolean DEFAULT false,
    raw text DEFAULT ''::text NOT NULL,
    vector tsvector
);


ALTER TABLE articles OWNER TO coyove;

--
-- Name: history_id_seq; Type: SEQUENCE; Schema: public; Owner: coyove
--

CREATE SEQUENCE history_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE history_id_seq OWNER TO coyove;

--
-- Name: history; Type: TABLE; Schema: public; Owner: coyove; Tablespace: 
--

CREATE TABLE history (
    id integer DEFAULT nextval('history_id_seq'::regclass) NOT NULL,
    article_id integer,
    date bigint,
    content text,
    user_id integer,
    title text,
    raw text DEFAULT ''::text NOT NULL
);


ALTER TABLE history OWNER TO coyove;

--
-- Name: image_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE image_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE image_id_seq OWNER TO postgres;

--
-- Name: image_keywords_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE image_keywords_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE image_keywords_id_seq OWNER TO postgres;

--
-- Name: image_keywords; Type: TABLE; Schema: public; Owner: postgres; Tablespace: 
--

CREATE TABLE image_keywords (
    keyword text NOT NULL,
    ts bigint DEFAULT ((date_part('epoch'::text, now()))::bigint * 1000),
    children integer DEFAULT 0,
    id integer DEFAULT nextval('image_keywords_id_seq'::regclass)
);


ALTER TABLE image_keywords OWNER TO postgres;

--
-- Name: images; Type: TABLE; Schema: public; Owner: postgres; Tablespace: 
--

CREATE TABLE images (
    id integer DEFAULT nextval('image_id_seq'::regclass) NOT NULL,
    image text,
    uploader integer,
    ts bigint DEFAULT ((date_part('epoch'::text, now()))::bigint * 1000) NOT NULL,
    hits integer DEFAULT 0 NOT NULL,
    path text DEFAULT ''::text NOT NULL,
    hide boolean DEFAULT false,
    filename text DEFAULT ''::text,
    requests integer DEFAULT 0,
    size integer DEFAULT 0,
    r18 boolean DEFAULT false
);


ALTER TABLE images OWNER TO postgres;

--
-- Name: tags; Type: TABLE; Schema: public; Owner: postgres; Tablespace: 
--

CREATE TABLE tags (
    id integer NOT NULL,
    name text,
    description text,
    restricted text,
    hidden boolean,
    short text,
    announce_id integer DEFAULT 0,
    children integer DEFAULT 0
);


ALTER TABLE tags OWNER TO postgres;

--
-- Name: users; Type: TABLE; Schema: public; Owner: postgres; Tablespace: 
--

CREATE TABLE users (
    id integer NOT NULL,
    username text,
    password text,
    signup_date timestamp with time zone DEFAULT now(),
    last_login_date timestamp with time zone DEFAULT now(),
    last_last_login_date timestamp with time zone DEFAULT now(),
    session_id text DEFAULT ''::text,
    retry integer DEFAULT 0,
    lock_date timestamp with time zone DEFAULT (now() - '00:30:00'::interval),
    nickname text,
    last_login_ip text DEFAULT ''::text,
    last_last_login_ip text DEFAULT ''::text,
    password_hint text DEFAULT ''::text
);


ALTER TABLE users OWNER TO postgres;

--
-- Name: user_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE user_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE user_id_seq OWNER TO postgres;

--
-- Name: user_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE user_id_seq OWNED BY users.id;


--
-- Name: user_info; Type: TABLE; Schema: public; Owner: postgres; Tablespace: 
--

CREATE TABLE user_info (
    id integer NOT NULL,
    username text,
    status character(8) DEFAULT 'ok'::text,
    "group" character(16) DEFAULT 'user'::bpchar,
    avatar text DEFAULT 'null'::text,
    image_usage integer DEFAULT 0,
    g_visible text DEFAULT 'user'::text,
    index integer DEFAULT 0
);


ALTER TABLE user_info OWNER TO postgres;

--
-- Name: id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY users ALTER COLUMN id SET DEFAULT nextval('user_id_seq'::regclass);


--
-- Name: articles_pkey; Type: CONSTRAINT; Schema: public; Owner: coyove; Tablespace: 
--

ALTER TABLE ONLY articles
    ADD CONSTRAINT articles_pkey PRIMARY KEY (id);


--
-- Name: history_pkey; Type: CONSTRAINT; Schema: public; Owner: coyove; Tablespace: 
--

ALTER TABLE ONLY history
    ADD CONSTRAINT history_pkey PRIMARY KEY (id);


--
-- Name: image_keywords_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres; Tablespace: 
--

ALTER TABLE ONLY image_keywords
    ADD CONSTRAINT image_keywords_pkey PRIMARY KEY (keyword);


--
-- Name: tags_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres; Tablespace: 
--

ALTER TABLE ONLY tags
    ADD CONSTRAINT tags_pkey PRIMARY KEY (id);


--
-- Name: threads_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres; Tablespace: 
--

ALTER TABLE ONLY images
    ADD CONSTRAINT threads_pkey PRIMARY KEY (id);


--
-- Name: user_info_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres; Tablespace: 
--

ALTER TABLE ONLY user_info
    ADD CONSTRAINT user_info_pkey PRIMARY KEY (id);


--
-- Name: users_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres; Tablespace: 
--

ALTER TABLE ONLY users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: articles_created_at_index; Type: INDEX; Schema: public; Owner: coyove; Tablespace: 
--

CREATE INDEX articles_created_at_index ON articles USING btree (created_at);


--
-- Name: articles_modified_at_index; Type: INDEX; Schema: public; Owner: coyove; Tablespace: 
--

CREATE INDEX articles_modified_at_index ON articles USING btree (modified_at);


--
-- Name: articles_raw_bigm_index; Type: INDEX; Schema: public; Owner: coyove; Tablespace: 
--

CREATE INDEX articles_raw_bigm_index ON articles USING gin (raw gin_bigm_ops);


--
-- Name: articles_tag_index; Type: INDEX; Schema: public; Owner: coyove; Tablespace: 
--

CREATE INDEX articles_tag_index ON articles USING btree (tag);


--
-- Name: articles_title_bigm_index; Type: INDEX; Schema: public; Owner: coyove; Tablespace: 
--

CREATE INDEX articles_title_bigm_index ON articles USING gin (title gin_bigm_ops);


--
-- Name: filename_index; Type: INDEX; Schema: public; Owner: postgres; Tablespace: 
--

CREATE INDEX filename_index ON images USING gin (filename gin_bigm_ops);


--
-- Name: history_article_id_index; Type: INDEX; Schema: public; Owner: coyove; Tablespace: 
--

CREATE INDEX history_article_id_index ON history USING btree (article_id);


--
-- Name: image_index; Type: INDEX; Schema: public; Owner: postgres; Tablespace: 
--

CREATE INDEX image_index ON images USING btree (image);


--
-- Name: ts_index; Type: INDEX; Schema: public; Owner: postgres; Tablespace: 
--

CREATE INDEX ts_index ON images USING btree (ts);


--
-- Name: users_nickname_index; Type: INDEX; Schema: public; Owner: postgres; Tablespace: 
--

CREATE INDEX users_nickname_index ON users USING btree (nickname);


--
-- Name: users_username_index; Type: INDEX; Schema: public; Owner: postgres; Tablespace: 
--

CREATE INDEX users_username_index ON users USING btree (username);


--
-- Name: image_keywords_ignore_duplicate_keywords; Type: RULE; Schema: public; Owner: postgres
--

CREATE RULE image_keywords_ignore_duplicate_keywords AS
    ON INSERT TO image_keywords
   WHERE (EXISTS ( SELECT 1
           FROM image_keywords image_keywords_1
          WHERE (image_keywords_1.keyword = new.keyword))) DO INSTEAD  UPDATE image_keywords SET children = (image_keywords.children + 1)
  WHERE (image_keywords.keyword = new.keyword);


--
-- Name: filename_insert_trigger; Type: TRIGGER; Schema: public; Owner: postgres
--

CREATE TRIGGER filename_insert_trigger AFTER INSERT ON images FOR EACH ROW EXECUTE PROCEDURE filename_changed();


--
-- Name: filename_trigger; Type: TRIGGER; Schema: public; Owner: postgres
--

CREATE TRIGGER filename_trigger AFTER UPDATE OF filename ON images FOR EACH ROW EXECUTE PROCEDURE filename_changed();


--
-- Name: new_user_registered; Type: TRIGGER; Schema: public; Owner: postgres
--

CREATE TRIGGER new_user_registered AFTER INSERT ON users FOR EACH ROW EXECUTE PROCEDURE new_user_registered();


--
-- Name: articles_author_fkey; Type: FK CONSTRAINT; Schema: public; Owner: coyove
--

ALTER TABLE ONLY articles
    ADD CONSTRAINT articles_author_fkey FOREIGN KEY (author) REFERENCES user_info(id);


--
-- Name: public; Type: ACL; Schema: -; Owner: postgres
--

REVOKE ALL ON SCHEMA public FROM PUBLIC;
REVOKE ALL ON SCHEMA public FROM postgres;
GRANT ALL ON SCHEMA public TO postgres;
GRANT ALL ON SCHEMA public TO PUBLIC;


--
-- Name: new_user_registered(); Type: ACL; Schema: public; Owner: postgres
--

REVOKE ALL ON FUNCTION new_user_registered() FROM PUBLIC;
REVOKE ALL ON FUNCTION new_user_registered() FROM postgres;
GRANT ALL ON FUNCTION new_user_registered() TO postgres;
GRANT ALL ON FUNCTION new_user_registered() TO PUBLIC;
GRANT ALL ON FUNCTION new_user_registered() TO coyove;


--
-- Name: article_id_seq; Type: ACL; Schema: public; Owner: coyove
--

REVOKE ALL ON SEQUENCE article_id_seq FROM PUBLIC;
REVOKE ALL ON SEQUENCE article_id_seq FROM coyove;
GRANT ALL ON SEQUENCE article_id_seq TO coyove;


--
-- Name: articles; Type: ACL; Schema: public; Owner: coyove
--

REVOKE ALL ON TABLE articles FROM PUBLIC;
REVOKE ALL ON TABLE articles FROM coyove;
GRANT ALL ON TABLE articles TO coyove;


--
-- Name: history_id_seq; Type: ACL; Schema: public; Owner: coyove
--

REVOKE ALL ON SEQUENCE history_id_seq FROM PUBLIC;
REVOKE ALL ON SEQUENCE history_id_seq FROM coyove;
GRANT ALL ON SEQUENCE history_id_seq TO coyove;


--
-- Name: history; Type: ACL; Schema: public; Owner: coyove
--

REVOKE ALL ON TABLE history FROM PUBLIC;
REVOKE ALL ON TABLE history FROM coyove;
GRANT ALL ON TABLE history TO coyove;


--
-- Name: image_id_seq; Type: ACL; Schema: public; Owner: postgres
--

REVOKE ALL ON SEQUENCE image_id_seq FROM PUBLIC;
REVOKE ALL ON SEQUENCE image_id_seq FROM postgres;
GRANT ALL ON SEQUENCE image_id_seq TO postgres;
GRANT ALL ON SEQUENCE image_id_seq TO coyove;


--
-- Name: images; Type: ACL; Schema: public; Owner: postgres
--

REVOKE ALL ON TABLE images FROM PUBLIC;
REVOKE ALL ON TABLE images FROM postgres;
GRANT ALL ON TABLE images TO postgres;
GRANT ALL ON TABLE images TO coyove;


--
-- Name: users; Type: ACL; Schema: public; Owner: postgres
--

REVOKE ALL ON TABLE users FROM PUBLIC;
REVOKE ALL ON TABLE users FROM postgres;
GRANT ALL ON TABLE users TO postgres;
GRANT ALL ON TABLE users TO coyove;


--
-- Name: user_id_seq; Type: ACL; Schema: public; Owner: postgres
--

REVOKE ALL ON SEQUENCE user_id_seq FROM PUBLIC;
REVOKE ALL ON SEQUENCE user_id_seq FROM postgres;
GRANT ALL ON SEQUENCE user_id_seq TO postgres;
GRANT ALL ON SEQUENCE user_id_seq TO coyove;


--
-- Name: user_info; Type: ACL; Schema: public; Owner: postgres
--

REVOKE ALL ON TABLE user_info FROM PUBLIC;
REVOKE ALL ON TABLE user_info FROM postgres;
GRANT ALL ON TABLE user_info TO postgres;
GRANT ALL ON TABLE user_info TO coyove;


--
-- PostgreSQL database dump complete
--

