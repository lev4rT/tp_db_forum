DROP TABLE IF EXISTS users CASCADE;
CREATE TABLE users (
                       nickname TEXT NOT NULL UNIQUE PRIMARY KEY,  -- Имя пользователя (уникальное поле). Данное поле допускает только латиницу, цифры и знак подчеркивания. Сравнение имени регистронезависимо.
                       fullname TEXT NOT NULL,  -- Полное имя пользователя.
                       about TEXT,  -- Описание пользователя.
                       email TEXT NOT NULL UNIQUE  -- Почтовый адрес пользователя (уникальное поле).
);

DROP TABLE IF EXISTS forums CASCADE;
CREATE TABLE forums (
                        title TEXT NOT NULL UNIQUE,  -- Название форума.
                        "user" TEXT NOT NULL REFERENCES users(nickname) ,  -- Nickname пользователя, который отвечает за форум.
                        slug TEXT NOT NULL UNIQUE PRIMARY KEY,  -- Человекопонятный URL. Уникальное поле.
                        posts BIGINT DEFAULT 0,  -- Общее кол-во сообщений в данном форуме.
                        threads BIGINT DEFAULT 0  -- Общее кол-во ветвей обсуждения в данном форуме.
);

DROP TABLE IF EXISTS threads CASCADE;
CREATE TABLE threads (
                         id SERIAL NOT NULL PRIMARY KEY,  -- Идентификатор ветки обсуждения.
                         title TEXT NOT NULL,  -- Заголовок ветки обсуждения.
                         author TEXT NOT NULL REFERENCES users(nickname),  -- Пользователь, создавший данную тему.
                         forum TEXT NOT NULL REFERENCES forums(slug),  -- Форум, в котором расположена данная ветка обсуждения.
                         message TEXT NOT NULL,  -- Описание ветки обсуждения.
                         votes INTEGER DEFAULT 0,  -- Кол-во голосов непосредственно за данное сообщение форума.
                         slug TEXT DEFAULT NULL,  -- Человекопонятный URL. В данной структуре slug опционален и не может быть числом.
                         created TIMESTAMP WITH TIME ZONE DEFAULT NOW(),  -- Дата создания ветки на форуме.

                         CONSTRAINT unique_thread UNIQUE (forum, author, title)
);

DROP TABLE IF EXISTS posts CASCADE;
CREATE TABLE posts (
                       id BIGSERIAL NOT NULL PRIMARY KEY,  -- Идентификатор данного сообщения.
                       parent BIGINT REFERENCES posts(id) DEFAULT 0,  -- Идентификатор родительского сообщения (0 - корневое сообщение обсуждения).
                       author TEXT NOT NULL REFERENCES users(nickname),  -- Автор, написавший данное сообщение.
                       message TEXT NOT NULL,  -- Собственно сообщение форума.
                       isEdited BOOLEAN DEFAULT false,  -- Истина, если данное сообщение было изменено.
                       forum TEXT NOT NULL REFERENCES forums(slug),  -- Идентификатор форума (slug) данного сообещния.
                       thread INTEGER REFERENCES threads(id),  -- Идентификатор ветви (id) обсуждения данного сообещния.
                       created TIMESTAMP WITH TIME ZONE DEFAULT NOW()  -- Дата создания сообщения на форуме.
);

DROP TABLE IF EXISTS votes CASCADE;
CREATE TABLE votes(
                      nickname TEXT NOT NULL REFERENCES users(nickname),  -- Идентификатор пользователя.
                      voice SMALLINT,  -- Отданный голос.
                      threadID INT REFERENCES threads(id),  -- ID  треда

                      CONSTRAINT unique_vote UNIQUE (nickname, threadID)
);

CREATE OR REPLACE FUNCTION addVoteForThread() RETURNS TRIGGER AS
$$
BEGIN
    UPDATE threads SET votes = votes + NEW."voice" WHERE id = NEW."threadid";
    RETURN NEW;
END;
$$
LANGUAGE 'plpgsql';

CREATE TRIGGER addVoteForThreadTrigger AFTER INSERT ON "votes"
    FOR EACH ROW
    EXECUTE PROCEDURE addVoteForThread();

------------------------------------------------------------------------

CREATE OR REPLACE FUNCTION changeVoteForThread() RETURNS TRIGGER AS
$$
BEGIN
    UPDATE threads SET votes = votes + 2 * NEW."voice" WHERE id = NEW."threadid";
    RETURN NEW;
END;
$$
LANGUAGE 'plpgsql';

CREATE TRIGGER changeVoteForThreadTrigger AFTER UPDATE ON "votes"
    FOR EACH ROW
    EXECUTE PROCEDURE changeVoteForThread();